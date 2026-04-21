package mirea

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/publicsuffix"
	"log"
	"mirea-qr/internal/config"
	"mirea-qr/internal/entity"
	"mirea-qr/pkg/customerrors"
	message "mirea-qr/pkg/mirea/proto"
	"mirea-qr/pkg/proxy"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/redis/go-redis/v9"
	"resty.dev/v3"
)

var proxyBlockedErr = errors.New("proxy blocked")

// URL-ы MIREA, обновлено на pulse.mirea.ru
var mireaSessionURLs = []*url.URL{
	mustParseURL("https://attendance.mirea.ru"),
	mustParseURL("https://pulse.mirea.ru"),
	mustParseURL("https://attendance.mirea.ru/api/mireaauth"),
	mustParseURL("https://sso.mirea.ru/realms/mirea/"),
}

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

type Attendance struct {
	config       *config.Config
	user         entity.User
	appVersion   string
	client       *resty.Client
	redis        *redis.Client
	useCache     bool
	currentProxy string
	retryCount   int
}

type RestySession struct {
	Cookies []*http.Cookie `json:"cookies"`
}

type GroupResponse struct {
	UUID  string
	Title string
}

func NewAttendance(cfg *config.Config, user entity.User, redis *redis.Client) *Attendance {
	client := resty.New()
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	client.SetCookieJar(jar)
	client.SetHeader("User-Agent", user.UserAgent)
	client.SetTimeout(30 * time.Second)

	a := &Attendance{
		config:     cfg,
		user:       user,
		appVersion: "1.7.0+5808",
		client:     client,
		redis:      redis,
		useCache:   true,
		retryCount: 0,
	}

	if a.config.UseProxy {
		a.SetProxy()
	}

	return a
}

func (a *Attendance) SetProxy() string {
	randProxy, err := proxy.GetUserProxy(a.user.CustomProxy, a.redis)
	if err != nil {
		log.Printf("[PROXY WARNING] failed load proxy : %+v", err)
		return ""
	}
	if randProxy != "" {
		a.client.SetProxy(randProxy)
		a.currentProxy = randProxy
	}
	return randProxy
}

func (a *Attendance) SetUseCase(cache bool) {
	a.useCache = cache
}

func (a *Attendance) GetCurrentUser() entity.User {
	return a.user
}

func (a *Attendance) saveSessionToRedis() error {
	ctx := context.Background()
	jar := a.client.CookieJar()
	seen := make(map[string]struct{})
	var all []*http.Cookie
	for _, u := range mireaSessionURLs {

		for _, c := range jar.Cookies(u) {
			domain := u.Hostname()
			path := u.Path
			if path == "" {
				path = "/"
			}
			key := c.Name + "|" + domain + "|" + path
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			cp := *c
			cp.Domain = domain
			cp.Path = path
			cp.HttpOnly = true
			cp.Secure = true
			cp.MaxAge = 3600 * 24 * 400
			cp.SameSite = http.SameSiteNoneMode
			cp.Expires = time.Now().AddDate(1, 0, 0)
			all = append(all, &cp)

		}
	}

	if len(all) == 0 {
		return nil
	}

	session := RestySession{Cookies: all}
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return a.redis.Set(ctx, a.getSessionKeyToRedis(), data, 7*24*time.Hour).Err()
}

func (a *Attendance) loadSessionFromRedis() error {
	ctx := context.Background()
	data, err := a.redis.Get(ctx, a.getSessionKeyToRedis()).Bytes()

	if err != nil {
		return err
	}
	var session RestySession
	if err := json.Unmarshal(data, &session); err != nil {
		return err
	}
	jar := a.client.CookieJar()

	for _, c := range session.Cookies {
		if c == nil {
			continue
		}

		host := strings.TrimPrefix(strings.TrimSpace(c.Domain), ".")
		if host == "" {
			continue
		}
		path := c.Path
		if path == "" {
			path = "/"
		}
		if len(path) > 0 && path[0] != '/' {
			path = "/" + path
		}
		u, err := url.Parse("https://" + host + path)
		if err != nil {
			continue
		}
		jar.SetCookies(u, []*http.Cookie{c})
	}

	return nil
}

// Загрузка сессии без попытки авторизации (Для сканнера)
func (a *Attendance) LoadSessionOnly() error {
	err := a.loadSessionFromRedis()
	if err != nil {
		return err
	}

	u, _ := url.Parse("https://attendance.mirea.ru")
	cookies := a.client.CookieJar().Cookies(u)
	if len(cookies) == 0 {
		return errors.New("cookies missing in redis")
	}
	return nil
}

func (a *Attendance) getSessionKeyToRedis() string {
	return "sess_" + a.user.Email
}

func (a *Attendance) Authorization() error {
	if a.retryCount >= 3 {
		return customerrors.NewAuthError("network_error", "Сайт MIREA не отвечает", errors.New("network error"))
	}

	if a.useCache {
		if err := a.loadSessionFromRedis(); err == nil {
			info, err := a.GetMeInfo()
			if err != nil {
				if a.currentProxy != "" {
					_ = proxy.BlockProxy(a.redis, a.currentProxy, 30*time.Second)
				}
				if a.config.UseProxy {
					a.SetProxy()
				}
				a.retryCount++
				return a.Authorization()
			}
			if info != nil {
				return nil
			}
		}
	}

	if err := a.checkSiteAvailability(); err != nil {
		return err
	}

	// ОБНОВЛЕНО: redirectUri теперь указывает на pulse.mirea.ru
	resp, err := a.client.R().
		Get("https://attendance.mirea.ru/api/auth/login?redirectUri=https%3A%2F%2Fpulse.mirea.ru%2Fservices&rememberMe=True")
	if err != nil {
		if a.currentProxy != "" {
			_ = proxy.BlockProxy(a.redis, a.currentProxy, 30*time.Second)
		}
		if a.config.UseProxy {
			a.SetProxy()
		}
		a.retryCount++
		return a.Authorization()
	}

	redirects := resp.RedirectHistory()
	if len(redirects) == 0 {
		return customerrors.NewAuthError("site_error", "Ошибка редиректа", errors.New("no redirects in history"))
	}

	resp, err = a.client.
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_9_3) AppleWebKit/537.75.14 (KHTML, like Gecko) Version/7.0.3 Safari/7046A194A").
		R().
		Get(redirects[0].URL)
	if err != nil {
		if a.currentProxy != "" {
			_ = proxy.BlockProxy(a.redis, a.currentProxy, 30*time.Second)
		}
		if a.config.UseProxy {
			a.SetProxy()
		}
		a.retryCount++
		return a.Authorization()
	}

	loginActionURL, err := a.getLoginActionURL(resp.String())
	if err != nil {
		return err
	}

	loginResp, err := a.performLogin(loginActionURL)
	if err != nil {
		return err
	}

	redirects = loginResp.RedirectHistory()
	if len(redirects) == 0 {
		return customerrors.NewAuthError("invalid_credentials", "Неверный логин или пароль от MIREA", errors.New("not redirected after authorization"))
	}

	if err := a.saveSessionToRedis(); err != nil {
		return customerrors.NewAuthError("internal_error", "Система кеша не отвечает", err)
	}

	// ОБНОВЛЕНО: проверяем редирект на новый домен
	if redirects[0].URL != "https://pulse.mirea.ru/services" {
		loginRespStr := loginResp.String()
		if strings.Contains(loginRespStr, `"pageId": "email-code-form"`) ||
			strings.Contains(loginRespStr, `"pageId": "login-max-otp"`) {
			otpType := "email"
			if strings.Contains(loginRespStr, `"pageId": "login-max-otp"`) {
				otpType = "max"
			}

			loginActionURL, err = a.getLoginActionURL(loginRespStr)
			if err != nil {
				return err
			}

			return customerrors.NewAuthError(
				"otp_is_required",
				"Необходим OTP код",
				errors.New("opt code is required"),
			).SetLoginActionUrl(loginActionURL).SetOtpType(otpType)
		} else {
			return customerrors.NewAuthError("invalid_credentials", "Неверный логин или пароль от MIREA", errors.New("unexpected redirect, not two-factor auth"))
		}
	}

	group, err := a.GetAvailableGroup()
	if err != nil {
		return customerrors.NewAuthError("invalid_credentials", "Неверный логин или пароль от MIREA", errors.New("failed get group, because not authorized"))
	}
	a.user.GroupID = group.UUID

	return nil
}

func (a *Attendance) checkSiteAvailability() error {
	if a.retryCount >= 3 {
		return customerrors.NewAuthError("network_error", "Сайт MIREA не отвечает", errors.New("network error"))
	}

	// ОБНОВЛЕНО
	if _, err := a.client.R().Get("https://pulse.mirea.ru/"); err != nil {
		if a.currentProxy != "" {
			_ = proxy.BlockProxy(a.redis, a.currentProxy, 30*time.Second)
		}
		if a.config.UseProxy {
			a.SetProxy()
		}
		a.retryCount++
		return a.checkSiteAvailability()
	}
	return nil
}

func (a *Attendance) getLoginActionURL(resp string) (string, error) {
	re := regexp.MustCompile(`"loginAction": "(.*?)"`)
	match := re.FindStringSubmatch(resp)
	if len(match) < 2 {
		return "", customerrors.NewAuthError("site_error", "Ошибка получения ссылки авторизации с сайта MIREA", errors.New("login action not found"))
	}

	return match[1], nil
}

func (a *Attendance) performLogin(loginActionURL string) (*resty.Response, error) {
	resp, err := a.client.R().
		SetFormData(map[string]string{
			"username":     a.user.Email,
			"password":     a.user.Password,
			"rememberMe":   "on",
			"credentialId": "",
		}).
		SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*"+"/"+"*;q=0.8,application/signed-exchange;v=b3;q=0.7").
		SetHeader("Origin", "null").
		Post(loginActionURL)
	if err != nil {
		if a.currentProxy != "" {
			_ = proxy.BlockProxy(a.redis, a.currentProxy, 30*time.Second)
		}
		return nil, customerrors.NewAuthError("site_unavailable", "Сайт MIREA недоступен", err)
	}

	return resp, nil
}

func (a *Attendance) HandleTwoFactorAuth(loginActionURL string, code string, otpType string) error {

	err := a.loadSessionFromRedis()

	if err != nil {
		return customerrors.NewAuthError("session_error", "Ошибка получения сессии", err)
	}

	formData := map[string]string{}
	if otpType == "max" {
		formData = map[string]string{"code": code}
	} else {
		formData = map[string]string{"login": "true", "emailCode": code}
	}

	request := a.client.R().
		SetFormData(formData).
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_9_3) AppleWebKit/537.75.14 (KHTML, like Gecko) Version/7.0.3 Safari/7046A194A").
		SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*"+"/"+"*;q=0.8,application/signed-exchange;v=b3;q=0.7").
		SetHeader("Origin", "null")

	twoAuthResp, err := request.Post(loginActionURL)
	if err != nil {
		return customerrors.NewAuthError("site_unavailable", "Сайт MIREA недоступен", err)
	}

	if strings.Contains(twoAuthResp.String(), "\"summary\": \"Неверный код доступа.\"") {
		loginActionURL, err = a.getLoginActionURL(twoAuthResp.String())
		if err != nil {
			return customerrors.NewAuthError("site_unavailable", "Не удалось получить новый loginActionUrl", err)
		}

		return customerrors.NewAuthError("otp_code_is_wrong", "Неверный код OTP", errors.New("введен неверный OTP код")).
			SetLoginActionUrl(loginActionURL).SetOtpType(otpType)
	}

	if len(twoAuthResp.RedirectHistory()) == 0 {
		return customerrors.NewAuthError("site_unavailable", "Ошибка авторизации через OTP", errors.New("twoAuthResp не вернул redirectHistory"))
	}

	urlMax := twoAuthResp.RedirectHistory()[0].URL
	res, err := a.client.R().Get(urlMax)
	if err != nil {
		return err
	}

	loginActionURL, _ = a.getLoginActionURL(res.String())

	if loginActionURL != "" {
		res, err := a.client.R().
			SetFormData(map[string]string{"skip": "true", "retry": ""}).
			SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_9_3) AppleWebKit/537.75.14 (KHTML, like Gecko) Version/7.0.3 Safari/7046A194A").
			SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*"+"/"+"*;q=0.8,application/signed-exchange;v=b3;q=0.7").
			SetHeader("Origin", "null").
			Post(loginActionURL)

		if err != nil {
			return customerrors.NewAuthError("site_unavailable", "", err)
		}

		for _, redirect := range res.RedirectHistory() {
			_, _ = a.client.R().Get(redirect.URL)
		}
	}

	if err = a.saveSessionToRedis(); err != nil {
		return customerrors.NewAuthError("site_unavailable", "Ошибка сохранения сессии", err)
	}

	return nil
}

func (a *Attendance) makeGRPC(method string, request proto.Message, response proto.Message) error {
	data, err := proto.Marshal(request)
	if err != nil {
		return errors.New("failed marshal proto")
	}

	frame := make([]byte, 5)
	frame[0] = 0
	binary.BigEndian.PutUint32(frame[1:], uint32(len(data)))
	payload := append(frame, data...)

	// ОБНОВЛЕНО: Origin и Referer теперь указывают на Pulse
	resp, err := a.client.R().
		SetBody(payload).
		SetHeader("Accept", "*"+"/"+"*").
		SetHeader("Pulse-app-type", "pulse-app").
		SetHeader("Pulse-app-version", a.appVersion).
		SetHeader("Content-Type", "application/grpc-web+proto").
		SetHeader("Origin", "https://pulse.mirea.ru").
		SetHeader("Referer", "https://pulse.mirea.ru/").
		SetHeader("X-Grpc-Web", "1").
		SetHeader("X-Requested-With", "XMLHttpRequest").
		Post("https://attendance.mirea.ru/" + method)

	if err != nil {
		return err
	}

	// Дебаг-режим оставил, чтобы мы видели логи в случае проблем
	if strings.Contains(method, "SelfApproveAttendance") {
		fmt.Printf("\n========== DEBUG МИРЭА ==========\n")
		fmt.Printf("Метод: %s\n", method)
		fmt.Printf("Статус: %d\n", resp.StatusCode())
		fmt.Printf("Заголовки ответа: %+v\n", resp.Header())
		if len(resp.Bytes()) > 0 {
			fmt.Printf("Сырое тело ответа (String): %s\n", resp.String())
		} else {
			fmt.Printf("Тело ответа ПУСТОЕ!\n")
		}
		fmt.Printf("=================================\n\n")
	}

	if resp.StatusCode() == 401 {
		a.redis.Del(context.Background(), a.getSessionKeyToRedis())
		return errors.New("unauthorized")
	}
	if resp.StatusCode() == 403 {
		if a.currentProxy != "" {
			_ = proxy.BlockProxy(a.redis, a.currentProxy, 30*time.Second)
		}
		return proxyBlockedErr
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode(), method)
	}

	protoBytes, err := decodeGRPCBinaryResponse(resp.Bytes())
	if err != nil {
		return fmt.Errorf("decode error: %v", err)
	}

	if err := proto.Unmarshal(protoBytes, response); err != nil {
		return fmt.Errorf("proto unmarshal failed: %v", err)
	}

	return nil
}

func (a *Attendance) GetMeInfo() (*message.Student, error) {
	msg := &message.GetMeInfoRequest{
		Url:   "https://pulse.mirea.ru", // ОБНОВЛЕНО
		Value: 1,
	}

	response := &message.GetMeInfoResponse{}
	if err := a.makeGRPC("rtu_tc.rtu_attend.app.UserService/GetMeInfo", msg, response); err != nil {
		return nil, err
	}

	student := response.GetBody().GetStudent()

	return student, nil
}

func (a *Attendance) GetAvailableGroup() (*GroupResponse, error) {
	if a.user.Group == "" || string([]rune(a.user.Group)[:3]) == "ДПЗ" {
		groups, err := a.GetRelevantAcademicGroupsOfHuman(a.user.ID)
		if err == nil {
			for _, group := range groups {
				info, err := a.GetAcademicGroupInfo(group.GetUuid())
				if err != nil {
					continue
				}

				if info.GetDeparment().GetCode() == "ИДО" || string([]rune(group.GetTitle())[:3]) == "ДПЗ" {
					continue
				}

				a.user.Group = group.GetTitle()
				break
			}
		}
	}

	msg := &message.GetMeInfoRequest{
		Url:   "https://pulse.mirea.ru", // ОБНОВЛЕНО
		Value: 1,
	}

	response := &message.GetAvailableVisitingLogsOfStudentResponse{}
	if err := a.makeGRPC("rtu_tc.attendance.api.VisitingLogService/GetAvailableVisitingLogsOfStudent", msg, response); err != nil {
		return nil, err
	}

	if len(response.GetGroupData()) == 0 {
		return nil, errors.New("empty groups")
	}

	needTerm := response.GetGroupData()[0]

	for _, term := range response.GetGroupData() {
		if term.GetGroup().GetTitle() == a.user.Group && term.GetGroup().GetArchived() == 0 {
			needTerm = term
			break
		}
	}

	return &GroupResponse{
		UUID:  needTerm.GetGroup().GetUuid(),
		Title: needTerm.GetGroup().GetTitle(),
	}, nil
}

func (a *Attendance) GetLearnRatingScore() (*message.Response, error) {
	msg := &message.GetLearnRatingScoreRequest{
		Group: a.user.GroupID,
	}

	response := &message.GetLearnRatingScoreResponse{}
	if err := a.makeGRPC("rtu_tc.attendance.api.LearnRatingScoreService/GetLearnRatingScoreReportForStudentInVisitingLogV2", msg, response); err != nil {
		return nil, err
	}

	return response.GetResponse(), nil
}

func (a *Attendance) SelfApproveAttendance(token string) (*message.SelfApproveAttendanceResponse, error) {
	msg := &message.SelfApproveAttendanceRequest{
		Uuid: token,
	}

	response := &message.SelfApproveAttendanceResponse{}
	
	// 🎯 НОВОЕ ИМЯ МЕТОДА
	if err := a.makeGRPC("rtu_tc.attendance.api.StudentService/SelfApproveAttendanceThroughQRCode", msg, response); err != nil {
		return nil, err
	}

	return response, nil
}

func (a *Attendance) GetLessons(year, month, day int32) ([]*message.GetAvailableLessonsOfVisitingLogsResponse_Lesson, error) {
	msg := &message.GetAvailableLessonsOfVisitingLogsRequest{
		VisitingLogIds: a.user.GroupID,
		Date: &message.DateInfo{
			Year:  year,
			Month: month,
			Day:   day,
		},
	}

	response := &message.GetAvailableLessonsOfVisitingLogsResponse{}
	if err := a.makeGRPC("rtu_tc.attendance.api.LessonService/GetAvailableLessonsOfVisitingLogs", msg, response); err != nil {
		return nil, err
	}

	return response.GetLessons(), nil
}

func (a *Attendance) GetAttendanceStudentForLesson(lessonId string) ([]*message.AttendanceStudent, error) {
	msg := &message.GetAttendanceForLessonRequest{
		LessonId: lessonId,
		GroupId:  a.user.GroupID,
	}

	response := &message.GetAttendanceForLessonResponse{}
	if err := a.makeGRPC("rtu_tc.attendance.api.AttendanceService/GetAttendanceForLesson", msg, response); err != nil {
		return nil, err
	}

	return response.GetStudents(), nil
}

func (a *Attendance) GetHumanAcsEvents(startTime, endTime int64) ([]*message.GetHumanAcsEventsResponse_Info, error) {
	msg := &message.GetHumanAcsEventsRequest{
		StudentId: a.user.ID,
		TimeRange: &message.GetHumanAcsEventsRequest_TimeRange{
			StartTime: &message.GetHumanAcsEventsRequest_Time{Value: startTime},
			EndTime:   &message.GetHumanAcsEventsRequest_TimeTwo{Value: endTime, MegaHuinya: 999000000},
		},
		Huinya1: 1,
		Huinya2: 2,
	}

	response := &message.GetHumanAcsEventsResponse{}
	if err := a.makeGRPC("rtu_tc.rtu_attend.humanpass.HumanPassService/GetHumanAcsEvents", msg, response); err != nil {
		return nil, err
	}

	return response.GetInfo(), nil
}

func (a *Attendance) GetRelevantAcademicGroupsOfHuman(uuid string) ([]*message.GetRelevantAcademicGroupsOfHumanResponse_Group, error) {
	msg := &message.GetRelevantAcademicGroupsOfHumanRequest{
		Uuid: uuid,
	}

	response := &message.GetRelevantAcademicGroupsOfHumanResponse{}
	if err := a.makeGRPC("rtu_tc.student.api.AcademicGroupService/GetRelevantAcademicGroupsOfHuman", msg, response); err != nil {
		return nil, err
	}

	return response.GetGroups(), nil
}

func (a *Attendance) GetAcademicGroupInfo(uuid string) (*message.GetAcademicGroupInfoResponse, error) {
	msg := &message.GetAcademicGroupInfoRequest{
		Uuid: uuid,
	}

	response := &message.GetAcademicGroupInfoResponse{}
	if err := a.makeGRPC("rtu_tc.student.api.AcademicGroupService/GetAcademicGroupInfo", msg, response); err != nil {
		return nil, err
	}

	return response, nil
}

func (a *Attendance) makeGRPCBytes(method string, requestBytes []byte) ([]byte, error) {
	return a.makeGRPCBytesWithOrigin(method, requestBytes, "https://pulse.mirea.ru") // ОБНОВЛЕНО
}

func (a *Attendance) makeGRPCBytesWithOrigin(method string, requestBytes []byte, origin string) ([]byte, error) {
	frame := make([]byte, 5)
	frame[0] = 0
	binary.BigEndian.PutUint32(frame[1:], uint32(len(requestBytes)))
	payload := append(frame, requestBytes...)

	resp, err := a.client.R().
		SetBody(payload).
		SetHeader("Accept", "*"+"/"+"*").
		SetHeader("Pulse-app-type", "pulse-app").
		SetHeader("Pulse-app-version", a.appVersion).
		SetHeader("Content-Type", "application/grpc-web+proto").
		SetHeader("Origin", origin).
		SetHeader("Referer", origin+"/").
		SetHeader("X-Grpc-Web", "1").
		SetHeader("X-Requested-With", "XMLHttpRequest").
		Post("https://attendance.mirea.ru/" + method)

	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == 401 {
		a.redis.Del(context.Background(), a.getSessionKeyToRedis())
		return nil, errors.New("unauthorized")
	}
	if resp.StatusCode() == 403 {
		if a.currentProxy != "" {
			_ = proxy.BlockProxy(a.redis, a.currentProxy, 30*time.Second)
		}
		return nil, proxyBlockedErr
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode(), method)
	}

	return decodeGRPCBinaryResponse(resp.Bytes())
}

func decodeGRPCBinaryResponse(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty response")
	}
	offset := 0
	for offset+5 <= len(data) {
		flag := data[offset]
		length := binary.BigEndian.Uint32(data[offset+1 : offset+5])
		frameEnd := offset + 5 + int(length)
		if frameEnd > len(data) {
			return nil, fmt.Errorf("invalid gRPC frame: expected %d bytes, got %d", length, len(data)-offset-5)
		}
		if flag == 0x00 {
			return data[offset+5 : frameEnd], nil
		}
		offset = frameEnd
	}
	return nil, errors.New("no data frame in response")
}

func decodeGRPCResponseBytes(respString string) ([]byte, error) {
	respString = strings.TrimSpace(respString)
	respString = strings.ReplaceAll(respString, " ", "")
	respString = strings.ReplaceAll(respString, "\n", "")

	if respString == "" {
		return nil, errors.New("empty response")
	}

	re := regexp.MustCompile(`[A-Za-z0-9+/]+={0,2}`)
	matches := re.FindAllString(respString, -1)
	if len(matches) == 0 {
		return nil, errors.New("wrong base64")
	}

	decoded, err := base64.StdEncoding.DecodeString(matches[0])
	if err != nil {
		return nil, err
	}
	if len(decoded) < 6 {
		return nil, errors.New("response too short")
	}
	length := binary.BigEndian.Uint32(decoded[1:5])
	if uint32(len(decoded)-5) < length {
		return nil, fmt.Errorf("invalid gRPC frame: expected %d bytes, got %d", length, len(decoded)-5)
	}
	return decoded[5 : 5+length], nil
}

func decodeProtoResponse(respString string, respMessage proto.Message) error {
	respString = strings.TrimSpace(respString)
	respString = strings.ReplaceAll(respString, " ", "")
	respString = strings.ReplaceAll(respString, "\n", "")

	if respString == "" {
		return errors.New("empty response")
	}

	re := regexp.MustCompile(`[A-Za-z0-9+/]+={0,2}`)
	matches := re.FindAllString(respString, -1)
	if len(matches) == 0 {
		return errors.New("wrong base64")
	}
	resp := matches[0]
	resp = strings.TrimSpace(resp)
	resp = strings.ReplaceAll(resp, " ", "")
	resp = strings.ReplaceAll(resp, "\n", "")

	decoded, err := base64.StdEncoding.DecodeString(resp)
	if err != nil {
		return err
	}

	if len(decoded) < 6 {
		return errors.New("wrong base64")
	}
	length := binary.BigEndian.Uint32(decoded[1:5])
	if uint32(len(decoded)-5) < length {
		return errors.New(fmt.Sprintf("invalid length: expected %d, got %d", length, len(decoded)-5))
	}
	protobufData := decoded[5 : 5+length]

	if err := proto.Unmarshal(protobufData, respMessage); err != nil {
		return errors.New(fmt.Sprintf("proto unmarshal failed: %v", err))
	}

	return nil
}

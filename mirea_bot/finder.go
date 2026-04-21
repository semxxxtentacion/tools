package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
)

func main() {
	baseURL := "https://pulse.mirea.ru"

	fmt.Println("[*] Подключаемся к", baseURL)

	resp, err := http.Get(baseURL)
	if err != nil {
		fmt.Println("[!] Ошибка:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	pageContent := string(body)

	// ЖАДНАЯ РЕГУЛЯРКА: ищет вообще любые упоминания .js файлов в папке assets
	jsRegex := regexp.MustCompile(`(/assets/[a-zA-Z0-9_-]+\.js)`)
	matches := jsRegex.FindAllStringSubmatch(pageContent, -1)

	uniqueJS := make(map[string]bool)
	for _, m := range matches {
		uniqueJS[m[1]] = true
	}

	fmt.Printf("[*] Найдено %d JS-файлов (включая скрытые вендоры).\n", len(uniqueJS))

	// Ищем все gRPC методы формата rtu_tc.ЧТО_ТО.ЧТО_ТО/Метод
	grpcRegex := regexp.MustCompile(`rtu_tc\.[a-zA-Z0-9_.]+\/[a-zA-Z0-9_]+`)
	uniqueEndpoints := make(map[string]bool)

	for jsPath := range uniqueJS {
		jsURL := baseURL + jsPath
		fmt.Println("[*] Сканируем:", jsURL)
		
		jsResp, err := http.Get(jsURL)
		if err != nil {
			continue
		}
		jsBody, _ := io.ReadAll(jsResp.Body)
		jsResp.Body.Close()

		endpoints := grpcRegex.FindAllString(string(jsBody), -1)
		for _, ep := range endpoints {
			uniqueEndpoints[ep] = true
		}
	}

	fmt.Println("\n================ ВСЕ НАЙДЕННЫЕ МЕТОДЫ ================")
	for ep := range uniqueEndpoints {
		fmt.Println(ep)
	}
	fmt.Println("======================================================")
}

package mirea

import (
	"errors"
	"log"
	"strings"
	"time"
)

// encodeStringField encodes a protobuf length-delimited string field
func encodeStringField(fieldNum uint64, s string) []byte {
	b := []byte(s)
	tag := (fieldNum << 3) | 2 // wire type 2 = length-delimited
	var buf []byte
	buf = appendVarint(buf, tag)
	buf = appendVarint(buf, uint64(len(b)))
	buf = append(buf, b...)
	return buf
}

// encodeStudentTermPlansRequest encodes a GetStudentTermPlans request (field 1 = studentId)
func encodeStudentTermPlansRequest(studentId string) []byte {
	return encodeStringField(1, studentId)
}

// encodeAttendancesLRSRequest encodes a GetAttendancesAndLRSByEswpesIds request (field 1 = repeated eswpeIds)
func encodeAttendancesLRSRequest(eswpeIds []string) []byte {
	var buf []byte
	for _, id := range eswpeIds {
		buf = append(buf, encodeStringField(1, id)...)
	}
	return buf
}

// EswpePotential holds the parsed data from a single lrsEswpeReport
type EswpePotential struct {
	EswpeId         string
	PotentialScore  float64
	DisciplineTitle string
}

// parseAttendancesAndLRSResponse parses the raw protobuf response from GetAttendancesAndLRSByEswpesIds.
//
// Response structure:
//   - field 2 (repeated) = lrsSemesterReport
//   - field 4 (repeated) = lrsEswpeReport
//   - field 1 (string)   = eswpeId
//   - field 5 (varint)   = isDisciplineExcludedFromLrs (optional bool)
//   - field 6 (double)   = potentialScoreFromAttendance
//   - field 4 → field 1 → field 1 (string) = discipline title
func parseAttendancesAndLRSResponse(data []byte) []EswpePotential {
	var result []EswpePotential

	for _, sr := range getFields(data, 2) {
		if sr.wireType != 2 {
			continue
		}
		for _, er := range getFields(sr.bytes, 4) {
			if er.wireType != 2 {
				continue
			}

			// field 5 = isDisciplineExcludedFromLrs
			excludedFields := getFields(er.bytes, 5)
			if len(excludedFields) > 0 && excludedFields[0].varint != 0 {
				continue
			}

			// field 1 = eswpeId
			eswpeIdFields := getFields(er.bytes, 1)
			if len(eswpeIdFields) == 0 || eswpeIdFields[0].wireType != 2 {
				continue
			}
			eswpeId := string(eswpeIdFields[0].bytes)

			// field 6 = potentialScoreFromAttendance (wire type 1 = 64-bit double)
			potentialFields := getFields(er.bytes, 6)
			if len(potentialFields) == 0 || potentialFields[0].wireType != 1 {
				continue
			}
			potential := fixed64AsFloat64(potentialFields[0].bytes64)

			// field 4 → field 1 → field 1 = discipline title
			disciplineTitle := ""
			for _, dr := range getFields(er.bytes, 4) {
				if dr.wireType != 2 {
					continue
				}
				for _, inner1 := range getFields(dr.bytes, 1) {
					if inner1.wireType != 2 {
						continue
					}
					for _, inner2 := range getFields(inner1.bytes, 1) {
						if inner2.wireType == 2 && len(inner2.bytes) > 0 {
							disciplineTitle = string(inner2.bytes)
							break
						}
					}
					if disciplineTitle != "" {
						break
					}
				}
				if disciplineTitle != "" {
					break
				}
			}

			result = append(result, EswpePotential{
				EswpeId:         eswpeId,
				PotentialScore:  potential,
				DisciplineTitle: disciplineTitle,
			})
		}
	}
	return result
}

// getStudentAcademicId resolves the student's academic system UUID via GetStudentsOfHuman.
// The attendance system UUID (a.user.ID) differs from the academic planning system UUID.
func (a *Attendance) getStudentAcademicId() (string, error) {
	reqBytes := encodeStringField(1, a.user.ID) // field 1 = humanId
	data, err := a.makeGRPCBytes("rtu_tc.student.api.StudentService/GetStudentsOfHuman", reqBytes)
	if err != nil {
		return "", err
	}
	ids := extractUUIDs(data)
	// Return first UUID that differs from the attendance UUID
	for _, id := range ids {
		if id != a.user.ID {
			return id, nil
		}
	}
	if len(ids) == 0 {
		return "", errors.New("no student UUID in GetStudentsOfHuman response")
	}
	return ids[0], nil
}

// extractTermPlanEswpeIds extracts eswpeIds from a single termPlan's bytes.
// Path: termPlan → field2 → field1[i] → field1 = eswpeId
func extractTermPlanEswpeIds(tpBytes []byte) []string {
	f2flds := getFields(tpBytes, 2)
	if len(f2flds) == 0 || f2flds[0].wireType != 2 {
		return nil
	}
	inner := f2flds[0].bytes
	var ids []string
	for _, entry := range getFields(inner, 1) {
		if entry.wireType != 2 {
			continue
		}
		idFlds := getFields(entry.bytes, 1)
		if len(idFlds) == 0 || idFlds[0].wireType != 2 {
			continue
		}
		s := string(idFlds[0].bytes)
		if uuidRegex.MatchString(s) {
			ids = append(ids, s)
		}
	}
	return ids
}

// GetIdealAttendancePotentials fetches potentialScoreFromAttendance for the student's
// current semester disciplines by trying each termPlan until LRS returns data.
// Retries once on transient network errors (connection reset).
func (a *Attendance) GetIdealAttendancePotentials() (map[string]float64, error) {
	const maxAttempts = 2
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(500 * time.Millisecond)
		}
		result, err := a.getIdealAttendancePotentialsOnce()
		if err == nil {
			return result, nil
		}
		lastErr = err
		// Only retry on transient network errors
		if !strings.Contains(err.Error(), "connection reset") &&
			!strings.Contains(err.Error(), "EOF") &&
			!strings.Contains(err.Error(), "timeout") {
			break
		}
		log.Printf("[ideal] attempt %d failed with network error, retrying: %v", attempt+1, err)
	}
	return nil, lastErr
}

func (a *Attendance) getIdealAttendancePotentialsOnce() (map[string]float64, error) {
	studentId, err := a.getStudentAcademicId()
	if err != nil {
		log.Printf("[ideal] getStudentAcademicId failed: %v, falling back to user.ID", err)
		studentId = a.user.ID
	}

	reqBytes := encodeStudentTermPlansRequest(studentId)
	data, err := a.makeGRPCBytes("rtu_tc.student.api.StudentService/GetStudentTermPlans", reqBytes)
	if err != nil {
		return nil, err
	}

	// Try each termPlan separately until LRS returns data for the active semester.
	// LRS throws an exception when receiving IDs from a non-active semester.
	termPlans := getFields(data, 1)
	log.Printf("[ideal] user=%s found %d term plans", studentId, len(termPlans))
	for tpIdx, tp := range termPlans {
		if tp.wireType != 2 {
			continue
		}
		ids := extractTermPlanEswpeIds(tp.bytes)
		if len(ids) == 0 {
			log.Printf("[ideal] term plan %d: no eswpe ids", tpIdx)
			continue
		}
		potentials, err := a.GetAttendancesAndLRSPotential(ids)
		if err != nil {
			log.Printf("[ideal] term plan %d (%d ids): LRS error: %v", tpIdx, len(ids), err)
			continue
		}
		if len(potentials) == 0 {
			log.Printf("[ideal] term plan %d (%d ids): LRS returned empty", tpIdx, len(ids))
			continue
		}
		return potentials, nil
	}
	return nil, errors.New("no active semester found in term plans")
}

// GetAttendancesAndLRSPotential fetches potentialScoreFromAttendance for each discipline,
// keyed by discipline title.
func (a *Attendance) GetAttendancesAndLRSPotential(eswpeIds []string) (map[string]float64, error) {
	if len(eswpeIds) == 0 {
		return nil, errors.New("no eswpe ids provided")
	}
	reqBytes := encodeAttendancesLRSRequest(eswpeIds)
	data, err := a.makeGRPCBytesWithOrigin(
		"rtu_tc.attendance.api.LearnRatingScoreService/GetAttendancesAndLRSByEswpesIds",
		reqBytes,
		"https://pulse.mirea.ru",
	)
	if err != nil {
		return nil, err
	}

	potentials := parseAttendancesAndLRSResponse(data)
	result := make(map[string]float64, len(potentials))
	for _, p := range potentials {
		if p.DisciplineTitle != "" && p.PotentialScore > 0 {
			result[p.DisciplineTitle] = p.PotentialScore
		}
	}
	return result, nil
}

package middleware

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ehgks0000/rf-server-go/utils"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var LOGFILEBASE = "./logs/info/"
var _log *log.Logger
var _f *os.File
var _today time.Time = time.Now()

func init() {

	var err error

	err = godotenv.Load()
	if err != nil {
		log.Panicf("Error loading .env file")
		// os.Exit(1)
	}

	env := os.Getenv("APP_ENV")

	var infoLogFile = LOGFILEBASE + time.Now().Format("2006-01-02") + ".log"

	// Check if the directory exists, if not create it
	if _, err := os.Stat(LOGFILEBASE); os.IsNotExist(err) {
		err := os.MkdirAll(LOGFILEBASE, 0755)
		if err != nil {
			log.Panicf("error creating directory: %v", err)
		}
	}

	_f, err = os.OpenFile(infoLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Panicf("error opening file: %v", err)
	}

	var writer io.Writer

	if env == "prod" {
		// 프로덕션 환경: 로그 파일에만 기록
		writer = _f
	} else {
		// 개발 환경: 로그 파일과 터미널에 기록
		writer = io.MultiWriter(_f, os.Stdout)
	}
	_log = log.New(writer, "INFO ", log.LstdFlags|log.Lmicroseconds)
	_log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	go loop()
}

func maskPassword(body []byte) ([]byte, error) {
	var data map[string]interface{}

	err := json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	if password, exists := data["password"]; exists && password != nil {
		data["password"] = "******"
		maskedBody, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		return maskedBody, nil
	}

	return body, nil
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// this if block is where important things happen for rotation
		// changing output file for logger
		if !dateEqual(_today, time.Now()) {
			_today = time.Now()

			dailyLogFile := LOGFILEBASE + time.Now().Format("2006-01-02") + ".log"
			newF, err := os.OpenFile(dailyLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				log.Panicf("error opening file: %v", err)
			}
			wr := io.MultiWriter(newF, os.Stdout)
			_log.SetOutput(wr)
		}

		var bs string
		if c.Request.Method == "POST" || c.Request.Method == "PUT" {
			body, _ := io.ReadAll(c.Request.Body)
			maskedBody, err := maskPassword(body)
			if err != nil {
				c.String(http.StatusInternalServerError, "Internal Server Error")
				return
			}
			bs = string(maskedBody)
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}

		_log.Println(c.ClientIP(), c.Request.Method, c.Request.RequestURI, "REQUEST", bs)

		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		c.Next()
		// after this 'c.Next()' part is for request response logs
		// i standartize responses with a struct called ResponseModel
		// and unmarshalled to get the response message

		mdl := utils.ResponseModel{}

		if blw.Status() > 201 {
			resp, err := io.ReadAll(blw.body)
			if err == nil {
				json.Unmarshal(resp, &mdl)
			}
		}

		_log.Println(c.ClientIP(), c.Request.Method, c.Request.RequestURI, "RESPONSE", blw.Status(), mdl.Message)
		// go _log.Println(c.Request.RemoteAddr, c.Request.Method, c.Request.RequestURI, "RESPONSE", blw.Status(), mdl.Message)
	}
}

func dateEqual(date1, date2 time.Time) bool {
	y1, m1, d1 := date1.Date()
	y2, m2, d2 := date2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// CompressYesterdayLogs 함수는 지정된 디렉토리의 로그 파일을 검사하고,
// 오늘 생성된 것이 아닌 로그 파일을 .gz로 압축합니다.
func compressYesterdayLogs(directory string) error {
	today := time.Now().Format("2006-01-02")

	return filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil // 디렉토리면 스킵
		}

		filename := info.Name()
		fileExt := filepath.Ext(filename)
		fileDate := strings.TrimSuffix(filename, fileExt)

		// 파일의 날짜가 오늘과 다르면 압축
		if fileDate != today && fileExt != ".gz" {
			// 입력 파일 열기
			inputFile, err := os.Open(path)
			if err != nil {
				return err
			}
			defer inputFile.Close()

			// 출력 파일(.gz) 생성
			outputFile, err := os.Create(fmt.Sprintf("%s.gz", path))
			if err != nil {
				return err
			}
			defer outputFile.Close()

			// Gzip writer 생성
			writer := gzip.NewWriter(outputFile)
			defer writer.Close()

			// 파일 내용 복사 및 압축
			_, err = io.Copy(writer, inputFile)
			if err != nil {
				return err
			}

			// 원본 로그 파일 삭제
			err = os.Remove(path)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

const maxLogFiles = 7

func keepLatestLogs(directory string) error {
	type logFile struct {
		Path string
		Date time.Time
	}

	var logFiles []logFile

	// 디렉토리 내의 파일들을 확인
	err := filepath.WalkDir(directory, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			name := d.Name()
			if strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".log.gz") {
				// 파일 이름에서 날짜 추출
				datePart := strings.Split(name, ".")[0]
				date, err := time.Parse("2006-01-02", datePart)
				if err == nil {
					logFiles = append(logFiles, logFile{Path: path, Date: date})
				}
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// 날짜를 기준으로 내림차순 정렬
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].Date.After(logFiles[j].Date)
	})

	// 가장 최신 7개를 제외한 나머지 파일 삭제
	if len(logFiles) > maxLogFiles {
		for _, lf := range logFiles[maxLogFiles:] {
			os.Remove(lf.Path)
		}
	}

	return nil
}

func loop() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := compressYesterdayLogs(LOGFILEBASE)
			if err != nil {
				// 오류 처리. 필요에 따라 로그를 출력하거나 다른 조치를 취할 수 있습니다.
				fmt.Println("Error compressing log files:", err)
			}

			err = keepLatestLogs(LOGFILEBASE)
			if err != nil {
				fmt.Println("Error deleteing log files:", err)
			}
		}
	}
}

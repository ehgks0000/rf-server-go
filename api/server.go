package api

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v2"
)

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}

type Server struct {
	es     *opensearch.Client
	db     *sql.DB
	router *gin.Engine
	logger *log.Logger
}

func NewServer() (*Server, error) {

	dataSourceName := os.Getenv("DATABASE")
	elkURL := os.Getenv("ELK_DB")

	// 로거 초기화
	logger := log.New(os.Stdout, "app: ", log.LstdFlags)

	// MySQL 데이터베이스 연결
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// MySQL 데이터베이스 연결 테스트
	err = db.Ping()
	if err != nil {
		logger.Fatalf("Database ping failed: %v", err)
	}

	// ES 연결
	esClient, err := opensearch.NewClient(opensearch.Config{
		Addresses: []string{elkURL},
	})
	if err != nil {
		logger.Fatalf("Opensearch failed: %v", err)
	}

	server := &Server{
		db:     db,
		es:     esClient,
		logger: logger,
	}

	server.setupRoutes()

	return server, nil
}

type createUserRequest struct {
	Username string `json:"username" binding:"required,alphanum"`
	Password string `json:"password" binding:"required,min=6"`
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

func (server *Server) Run(port string) {
	server.router.Run(port)
}

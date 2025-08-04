package handlers

import (
	"GoRoutine/internal/config"
	"GoRoutine/internal/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"net/http"
)

var validate *validator.Validate

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
}

type Handler struct {
	usecase interfaces.Usecases
}

// NewHandler создает новый экземпляр Handler со всеми зависимостями
func NewHandler(usecase interfaces.Usecases) *Handler {
	return &Handler{
		usecase: usecase,
	}
}

// ProvideRouter создает и настраивает маршруты
func ProvideRouter(h *Handler, cfg *config.Config) http.Handler {
	r := gin.Default()

	// CORS
	//r.Use(cors.New(cors.Config{
	//	AllowOrigins:     cfg.Server.AllowedOrigins,
	//	AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	//	AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
	//	ExposeHeaders:    []string{"Content-Length"},
	//	AllowCredentials: true,
	//}))

	baseRouter := r.Group("/api/v1")

	baseRouter.GET("/start", h.Start)
	baseRouter.GET("/status/:proc_id", h.GetStatus)

	return r
}

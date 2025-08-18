package handlers

import (
	"GoRoutine/internal/config"
	"GoRoutine/internal/interfaces"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
}

type Handler struct {
	usecase interfaces.Usecases
	cfg     *config.Config
}

// NewHandler создает новый экземпляр Handler со всеми зависимостями
func NewHandler(usecase interfaces.Usecases, cfg *config.Config) *Handler {
	return &Handler{
		usecase: usecase,
		cfg:     cfg,
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

	authorized := baseRouter.Group("/")
	authorized.Use(h.authMiddleware())
	{
		authorized.GET("/start", h.Start)
		authorized.POST("/start", h.StartWithFile)
		authorized.GET("/status/:proc_id", h.GetStatus)
		authorized.GET("/ids", h.GetAllProcessIDs)
		authorized.POST("/process_ai/:proc_id", h.ProcessAI)
		authorized.POST("/start_full_pipeline", h.StartFullPipeline)
	}

	return r
}

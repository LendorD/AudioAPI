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

	baseRouterV2 := r.Group("/api/v2")

	authorizedV2 := baseRouterV2.Group("/")
	authorizedV2.Use(h.authMiddleware())

	{
		authorizedV2.GET("/files", h.GetFilesName)
		authorizedV2.GET("/status/:proc_id", h.GetStatus)
		authorizedV2.GET("/ids", h.GetAllProcessIDs)
		authorizedV2.POST("/process-all-toxicity", h.ProcessAllWithToxicityAnalysis)
		authorizedV2.POST("/process-toxic-full-analysis", h.ProcessToxicFilesWithFullAnalysis)
		authorizedV2.POST("/process-single", h.ProcessSingleFileWithToxicity)
		// authorizedV2.POST("/start_toxicity_pipeline", h.StartToxicityAnalysisPipeline)
	}

	return r
}

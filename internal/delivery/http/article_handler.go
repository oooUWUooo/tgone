package http

import (
	"habr-rss-bot/internal/domain"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ArticleHandler struct {
	articleUsecase domain.ArticleUsecase
}

func NewArticleHandler(r *gin.Engine, u domain.ArticleUsecase) {
	handler := &ArticleHandler{
		articleUsecase: u,
	}

	api := r.Group("/api")
	{
		api.GET("/articles", handler.GetArticles)
		api.GET("/articles/category/:category", handler.GetByCategory)
	}
}

// GetArticles godoc
// @Summary Get latest articles
// @Description Get a list of the latest articles from all sources
// @Tags articles
// @Accept  json
// @Produce  json
// @Param limit query int false "Limit number of articles" default(10)
// @Success 200 {array} domain.Article
// @Router /api/articles [get]
func (h *ArticleHandler) GetArticles(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)

	articles, err := h.articleUsecase.GetLatestArticles(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, articles)
}

// GetByCategory godoc
// @Summary Get articles by category
// @Description Get a list of articles from a specific category
// @Tags articles
// @Accept  json
// @Produce  json
// @Param category path string true "Category name"
// @Param limit query int false "Limit number of articles" default(10)
// @Success 200 {array} domain.Article
// @Router /api/articles/category/{category} [get]
func (h *ArticleHandler) GetByCategory(c *gin.Context) {
	category := c.Param("category")
	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)

	articles, err := h.articleUsecase.GetArticlesByCategory(category, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, articles)
}

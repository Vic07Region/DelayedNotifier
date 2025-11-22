package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"DelayedNotifier/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type Handler struct {
	service domain.NotificationService
}

func NewHandlersSet(service domain.NotificationService) *Handler {
	return &Handler{
		service: service,
	}
}

type CreateRequest struct {
	Recipient   string `json:"recipient" validate:"required"`
	Channel     string `json:"channel" validate:"required"`
	Payload     string `json:"payload" validate:"required,jsonstr"`
	ScheduledAt string `json:"scheduled_at" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
}

var validate = validator.New()
var ErrResponceMessage = gin.H{"error": ""}

func jsonStringValidator(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	var js map[string]interface{}
	return json.Unmarshal([]byte(value), &js) == nil
}

func validationMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "обязательное поле"
	case "jsonstr":
		return "должно быть корректным JSON-объектом"
	case "datetime":
		return "некорректный формат даты (ожидается RFC3339)"
	default:
		return "некорректное значение"
	}
}

func init() {
	_ = validate.RegisterValidation("jsonstr", jsonStringValidator)
}

func (h *Handler) CreateNotificationHandler(c *gin.Context) {
	var req CreateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный JSON: " + err.Error()})
		return
	}

	if err := validate.Struct(req); err != nil {
		var verrs validator.ValidationErrors
		if errors.As(err, &verrs) {
			errorsMap := make(map[string]string)
			for _, e := range verrs {
				errorsMap[e.Field()] = validationMessage(e)
			}

			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Ошибка валидации",
				"errors":  errorsMap,
			})
			return
		}
	}

	sheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		ErrResponceMessage["error"] = "Время указано некорректно"
		c.JSON(http.StatusBadRequest, ErrResponceMessage)
		return
	}

	var params domain.CreateNotificationParams
	if err = json.Unmarshal([]byte(req.Payload), &params.Payload); err != nil {
		ErrResponceMessage["error"] = "Ошибка сериализации payload"
		c.JSON(http.StatusBadRequest, ErrResponceMessage)
		return
	}

	ch := domain.Channel(req.Channel)
	if !ch.IsValid() {
		ErrResponceMessage["error"] = fmt.Sprintf("Канал отправки %s не поддерживается", req.Channel)
		c.JSON(http.StatusBadRequest, ErrResponceMessage)
		return
	}
	params.Channel = ch
	params.Recipient = req.Recipient
	params.ScheduledAt = sheduledAt

	n, err := h.service.CreateNotification(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": n,
	})
}

func (h *Handler) GetNotificationHandler(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is invalid"})
		return
	}

	n, err := h.service.GetNotificationByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": NotificationResponse{
		ID:          n.ID,
		Recipient:   n.Recipient,
		Channel:     n.Channel.String(),
		Payload:     n.Payload,
		ScheduledAt: n.ScheduledAt,
		Status:      n.Status.String(),
		RetryCount:  n.RetryCount,
		CreatedAt:   n.CreatedAt,
		UpdatedAt:   n.UpdatedAt,
	}})
}

func (h *Handler) DeleteNotificationHandler(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is invalid"})
		return
	}

	err = h.service.Cancel(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": idStr + " cancelled"})
}

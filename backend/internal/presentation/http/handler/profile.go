package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/user"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
)

type GetPrivateProfileUseCase interface {
	Execute(
		context.Context,
		int64,
	) (entity.PrivateProfile, error)
}

type GetPublicProfileUseCase interface {
	Execute(
		context.Context,
		int64,
	) (entity.PublicProfile, error)
}

type UpdateProfileUseCase interface {
	Execute(context.Context, int64, user.UpdateProfileInput) (entity.PrivateProfile, error)
}

type ProfileHandler struct {
	private GetPrivateProfileUseCase
	public  GetPublicProfileUseCase
	update  UpdateProfileUseCase
}

func NewProfileHandler(
	private GetPrivateProfileUseCase,
	public GetPublicProfileUseCase,
	update UpdateProfileUseCase,
) *ProfileHandler {
	return &ProfileHandler{
		private: private,
		public:  public,
		update:  update,
	}
}

func (h *ProfileHandler) Update(c *gin.Context) {
	identity, ok := middleware.Identity(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthenticated", "A valid access token is required.")
		return
	}
	var request updateProfileRequest
	if !readJSON(c, &request) {
		return
	}
	profile, err := h.update.Execute(c.Request.Context(), identity.UserID, toUpdateProfileInput(request))
	if err != nil {
		profileReadError(c, err)
		return
	}
	response.Success(c, http.StatusOK, toPrivateProfileDTO(profile))
}

func toUpdateProfileInput(request updateProfileRequest) user.UpdateProfileInput {
	return user.UpdateProfileInput{
		FirstName: optionalStringInput(request.FirstName), LastName: optionalStringInput(request.LastName),
		DateOfBirth: optionalStringInput(request.DateOfBirth), PhoneNumber: optionalStringInput(request.PhoneNumber),
	}
}

func optionalStringInput(field optionalJSONField[string]) user.UpdateField[string] {
	result := user.UpdateField[string]{Set: field.Set}
	if field.Set && !field.Null {
		value := field.Value
		result.Value = &value
	}
	return result
}

type privateProfileDTO struct {
	ID          int64   `json:"id"`
	FirstName   string  `json:"first_name"`
	LastName    string  `json:"last_name"`
	Name        string  `json:"name"`
	DateOfBirth *string `json:"date_of_birth"`
	PhoneNumber *string `json:"phone_number"`
	AvatarURL   string  `json:"avatar_url"`
}

type publicProfileDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

func toPrivateProfileDTO(
	profile entity.PrivateProfile,
) privateProfileDTO {
	var dateOfBirth *string

	if profile.DateOfBirth != nil {
		formatted := profile.DateOfBirth.Format("2006-01-02")
		dateOfBirth = &formatted
	}

	return privateProfileDTO{
		ID:          profile.ID,
		FirstName:   profile.FirstName,
		LastName:    profile.LastName,
		Name:        profile.Name,
		DateOfBirth: dateOfBirth,
		PhoneNumber: profile.PhoneNumber,
		AvatarURL:   profile.AvatarURL,
	}
}

func toPublicProfileDTO(
	profile entity.PublicProfile,
) publicProfileDTO {
	return publicProfileDTO{
		ID:        profile.ID,
		Name:      profile.Name,
		AvatarURL: profile.AvatarURL,
	}
}

func (h *ProfileHandler) GetPrivate(c *gin.Context) {
	identity, ok := middleware.Identity(c)
	if !ok {
		response.Error(
			c,
			http.StatusUnauthorized,
			"unauthenticated",
			"A valid access token is required.",
		)
		return
	}

	profile, err := h.private.Execute(
		c.Request.Context(),
		identity.UserID,
	)
	if err != nil {
		profileReadError(c, err)
		return
	}
	response.Success(c, http.StatusOK, toPrivateProfileDTO(profile))
}

func (h *ProfileHandler) GetPublic(c *gin.Context) {
	if _, ok := middleware.Identity(c); !ok {
		response.Error(
			c,
			http.StatusUnauthorized,
			"unauthenticated",
			"A valid access token is required.",
		)
		return
	}

	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.Error(
			c,
			http.StatusBadRequest,
			"invalid_input",
			"The submitted information is invalid.",
		)
		return
	}
	profile, err := h.public.Execute(
		c.Request.Context(),
		userID,
	)
	if err != nil {
		profileReadError(c, err)
		return
	}

	response.Success(c, http.StatusOK, toPublicProfileDTO(profile))
}

func profileReadError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, entity.ErrInvalidInput):
		response.Error(
			c,
			http.StatusBadRequest,
			"invalid_input",
			"The submitted information is invalid.",
		)

	case errors.Is(err, entity.ErrUserNotFound):
		response.Error(
			c,
			http.StatusNotFound,
			"not_found",
			"User not found.",
		)
	case errors.Is(err, entity.ErrPhoneNumberTaken):
		response.Error(c, http.StatusConflict, "phone_number_taken", "Phone number is already in use.")

	default:
		response.Error(
			c,
			http.StatusInternalServerError,
			"internal_error",
			"Unable to complete the request.",
		)
	}
}

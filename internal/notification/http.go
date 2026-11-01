package notification

import (
	"context"
	"errors"
	"net/http"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

type DeviceCommands interface {
	Register(context.Context, identity.UserID, string, string) error
	Unregister(context.Context, identity.UserID, string) error
}

type Handler struct{ commands DeviceCommands }

func NewHandler(commands DeviceCommands) *Handler { return &Handler{commands: commands} }

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.CurrentUser(r.Context())
	if !ok {
		httpapi.WriteError(w, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	var input struct {
		Token    string `json:"token"`
		Platform string `json:"platform"`
	}
	if httpapi.ReadJSON(w, r, &input, httpapi.DefaultMaxBodyBytes) != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_push_device", "A push token and platform are required.")
		return
	}
	if err := h.commands.Register(r.Context(), user.ID(), input.Token, input.Platform); err != nil {
		writeDeviceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Unregister(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.CurrentUser(r.Context())
	if !ok {
		httpapi.WriteError(w, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if httpapi.ReadJSON(w, r, &input, httpapi.DefaultMaxBodyBytes) != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_push_device", "A push token is required.")
		return
	}
	if err := h.commands.Unregister(r.Context(), user.ID(), input.Token); err != nil {
		writeDeviceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeDeviceError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrInvalidDevice) {
		httpapi.WriteError(w, http.StatusUnprocessableEntity, "invalid_push_device", "The push device details are invalid.")
		return
	}
	httpapi.WriteError(w, http.StatusInternalServerError, "push_device_unavailable", "Controlled could not update this push device.")
}

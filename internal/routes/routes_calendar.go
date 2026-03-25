package routes

import (
	"net/http"

	"charm.land/log/v2"
	"github.com/damongolding/immich-kiosk/internal/calendar"
	"github.com/damongolding/immich-kiosk/internal/config"
	"github.com/damongolding/immich-kiosk/internal/templates/partials"
	"github.com/labstack/echo/v5"
)

// Calendar handles POST /calendar requests, returning upcoming ICS calendar events as HTML.
func Calendar(baseConfig *config.Config) echo.HandlerFunc {
	return func(c *echo.Context) error {
		requestData, err := InitializeRequestData(c, baseConfig)
		if err != nil {
			return err
		}
		if requestData == nil {
			log.Info("Refreshing clients")
			return nil
		}

		log.Debug(
			requestData.RequestID,
			"method", c.Request().Method,
			"path", c.Request().URL.String(),
		)

		cfg := baseConfig.Calendar
		if cfg.URL == "" {
			return c.NoContent(http.StatusNoContent)
		}

		events := calendar.UpcomingEvents(cfg.MaxEvents, cfg.DaysAhead)
		return Render(c, http.StatusOK, partials.CalendarEvents(events, baseConfig.SystemLang))
	}
}

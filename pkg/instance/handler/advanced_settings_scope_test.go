package instance_handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	instance_service "github.com/evolution-foundation/evolution-go/pkg/instance/service"
	"github.com/gin-gonic/gin"
)

type scopedSettingsService struct {
	instance_service.InstanceService
	reads, writes int
	id            string
}

func (s *scopedSettingsService) GetAdvancedSettings(id string) (*instance_model.AdvancedSettings, error) {
	s.reads++
	s.id = id
	return &instance_model.AdvancedSettings{}, nil
}

func (s *scopedSettingsService) UpdateAdvancedSettings(id string, settings *instance_model.AdvancedSettings) error {
	s.writes++
	s.id = id
	return nil
}

func TestSettingsRoutesEnforceAuthenticatedInstance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const owner = "00000000-0000-4000-8000-000000000001"
	const other = "00000000-0000-4000-8000-000000000002"
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for _, tc := range []struct {
			name, path string
			auth       bool
			want       int
		}{
			{"own", owner, true, http.StatusOK},
			{"other", other, true, http.StatusForbidden},
			{"invalid", "invalid", true, http.StatusBadRequest},
			{"missing auth", owner, false, http.StatusUnauthorized},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				s := &scopedSettingsService{}
				h := NewInstanceHandler(s, nil)
				r := gin.New()
				r.Use(func(c *gin.Context) {
					if tc.auth {
						c.Set("instance", &instance_model.Instance{Id: owner})
					}
				})
				r.GET("/instance/:instanceId/advanced-settings", h.GetAdvancedSettings)
				r.PUT("/instance/:instanceId/advanced-settings", h.UpdateAdvancedSettings)
				w := httptest.NewRecorder()
				req := httptest.NewRequest(method, "/instance/"+tc.path+"/advanced-settings", strings.NewReader(`{"ignoreStatus":true}`))
				req.Header.Set("Content-Type", "application/json")
				r.ServeHTTP(w, req)
				if w.Code != tc.want {
					t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
				}
				if tc.want != http.StatusOK && (s.reads != 0 || s.writes != 0) {
					t.Fatal("unauthorized request reached storage")
				}
				if tc.want == http.StatusOK && s.id != owner {
					t.Fatal("operation changed instance")
				}
			})
		}
	}
}

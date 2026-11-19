package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeTenantSite(t *testing.T) {
	tests := []struct {
		name         string
		tenantID     string
		siteID       string
		wantTenantID string
		wantSiteID   string
	}{
		{name: "neither", tenantID: "", siteID: "", wantTenantID: "", wantSiteID: ""},
		{name: "tenant only is kept, the organization's default site is used", tenantID: "2", siteID: "", wantTenantID: "2", wantSiteID: ""},
		{name: "tenant and site", tenantID: "2", siteID: "3", wantTenantID: "2", wantSiteID: "3"},
		{name: "site without tenant is dropped", tenantID: "", siteID: "3", wantTenantID: "", wantSiteID: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantID, siteID := normalizeTenantSite(tt.tenantID, tt.siteID)
			assert.Equal(t, tt.wantTenantID, tenantID)
			assert.Equal(t, tt.wantSiteID, siteID)
		})
	}
}

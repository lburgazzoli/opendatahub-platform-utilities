package main

import (
	"testing"

	"github.com/onsi/gomega"
)

func TestLoadOptions(t *testing.T) {
	t.Setenv("HELM_CHART", "environment-chart")

	g := gomega.NewWithT(t)
	defaults := new(options)
	defaults.ChartPath = "flag-chart"
	loaded, err := loadOptions(t.Context(), *defaults)

	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(loaded.ChartPath).To(gomega.Equal("environment-chart"))
	g.Expect(loaded.HealthProbeBindAddress).To(gomega.Equal(defaultHealthProbeBindAddress))
}

func TestLoadOptionsRequiresChart(t *testing.T) {
	t.Setenv("HELM_CHART", "")

	g := gomega.NewWithT(t)
	var defaults options
	loaded, err := loadOptions(t.Context(), defaults)

	g.Expect(loaded).To(gomega.BeZero())
	g.Expect(err).To(gomega.MatchError(ErrChartPathRequired))
}

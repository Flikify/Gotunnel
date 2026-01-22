package handler

import (
	"github.com/gin-gonic/gin"
)

// TrafficHandler 流量统计处理器
type TrafficHandler struct {
	app AppInterface
}

// NewTrafficHandler 创建流量统计处理器
func NewTrafficHandler(app AppInterface) *TrafficHandler {
	return &TrafficHandler{app: app}
}

// GetStats 获取流量统计
// @Summary 获取流量统计
// @Description 获取24小时和总流量统计
// @Tags 流量
// @Produce json
// @Security Bearer
// @Success 200 {object} Response
// @Router /api/traffic/stats [get]
func (h *TrafficHandler) GetStats(c *gin.Context) {
	store := h.app.GetTrafficStore()

	// 获取24小时流量
	in24h, out24h, err := store.Get24HourTraffic()
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	// 获取总流量
	inTotal, outTotal, err := store.GetTotalTraffic()
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{
		"traffic_24h": gin.H{
			"inbound":  in24h,
			"outbound": out24h,
		},
		"traffic_total": gin.H{
			"inbound":  inTotal,
			"outbound": outTotal,
		},
	})
}

// GetHourly 获取每小时流量
// @Summary 获取每小时流量
// @Description 获取最近N小时的流量记录
// @Tags 流量
// @Produce json
// @Security Bearer
// @Param hours query int false "小时数" default(24)
// @Success 200 {object} Response
// @Router /api/traffic/hourly [get]
func (h *TrafficHandler) GetHourly(c *gin.Context) {
	hours := 24

	store := h.app.GetTrafficStore()
	records, err := store.GetHourlyTraffic(hours)
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{
		"records": records,
	})
}

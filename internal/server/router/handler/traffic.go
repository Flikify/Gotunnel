package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/router/dto"
)

// TrafficHandler 流量统计处理器
type TrafficHandler struct {
	store db.TrafficStore
}

// NewTrafficHandler 创建流量统计处理器
func NewTrafficHandler(store db.TrafficStore) *TrafficHandler {
	return &TrafficHandler{store: store}
}

// GetStats 获取流量统计
// @Summary 获取流量统计
// @Description 获取24小时和总流量统计
// @Tags 流量
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=dto.TrafficStatsResponse}
// @Router /api/traffic/stats [get]
func (h *TrafficHandler) GetStats(c *gin.Context) {
	// 获取24小时流量
	in24h, out24h, err := h.store.Get24HourTraffic()
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	// 获取总流量
	inTotal, outTotal, err := h.store.GetTotalTraffic()
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, dto.TrafficStatsResponse{
		Traffic24h: dto.TrafficTotals{
			Inbound:  in24h,
			Outbound: out24h,
		},
		TrafficTotal: dto.TrafficTotals{
			Inbound:  inTotal,
			Outbound: outTotal,
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
// @Success 200 {object} Response{data=dto.HourlyTrafficResponse}
// @Router /api/traffic/hourly [get]
func (h *TrafficHandler) GetHourly(c *gin.Context) {
	hours := 24

	records, err := h.store.GetHourlyTraffic(hours)
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, dto.HourlyTrafficResponse{Records: records})
}

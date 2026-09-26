package data

import (
	"aipanyi/backend/db"
	"aipanyi/backend/models"
	"testing"
)

// @Author spark
// @Date 2026/1/23 17:39
// @Desc
//-----------------------------------------------------------------------------------

func TestAIResponseResultService_GetAIResponseResultList(t *testing.T) {
	db.Init("../../data/stock.db")
	service := NewAIResponseResultService()
	list, err := service.GetAIResponseResultList(models.AIResponseResultQuery{
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		return
	}
	t.Log(list)

}

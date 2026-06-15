package impl

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"legacy-video/internal/infrastructure/persistence/sqlqueries"
)

// QuestionDB 由外部注入，指向题库所在业务数据库。
var QuestionDB *gorm.DB

// questionRow 对应题库 SQL 查询返回的字段结构。
type questionRow struct {
	ID               uint64    `gorm:"column:id"`
	Source           string    `gorm:"column:source"`
	SourceQuestionID string    `gorm:"column:source_question_id"`
	Content          string    `gorm:"column:content"`
	Answer           string    `gorm:"column:answer"`
	Analysis         string    `gorm:"column:analysis"`
	Knowledge        string    `gorm:"column:knowledge"`
	Subject          string    `gorm:"column:subject"`
	Type             string    `gorm:"column:type"`
	Status           int16     `gorm:"column:status"`
	CreateTime       time.Time `gorm:"column:create_time"`
	UpdateTime       time.Time `gorm:"column:update_time"`
}

// toQuestionJSON 把题库行记录映射成统一的 HTTP JSON 返回结构。
func toQuestionJSON(r questionRow) gin.H {
	return gin.H{
		"id":                 r.ID,
		"source":             r.Source,
		"source_question_id": r.SourceQuestionID,
		"content":            r.Content,
		"answer":             r.Answer,
		"analysis":           r.Analysis,
		"knowledge":          r.Knowledge,
		"subject":            r.Subject,
		"type":               r.Type,
		"status":             r.Status,
		"create_time":        r.CreateTime.Unix(),
		"update_time":        r.UpdateTime.Unix(),
	}
}

// ListQuestions GET /api/question/list?page=1&page_size=20
func ListQuestions(c *gin.Context) {
	if QuestionDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "db not initialized"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	if err := QuestionDB.WithContext(c).Raw(sqlqueries.CountQuestionBankQuery).Scan(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询失败: " + err.Error()})
		return
	}

	rows := make([]questionRow, 0, pageSize)
	if err := QuestionDB.WithContext(c).Raw(sqlqueries.ListQuestionBankQuery, pageSize, offset).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询失败: " + err.Error()})
		return
	}

	list := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		list = append(list, toQuestionJSON(r))
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"list":      list,
	})
}

// GetQuestion GET /api/question/:id
func GetQuestion(c *gin.Context) {
	if QuestionDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "db not initialized"})
		return
	}

	id, ok := parseUint64Param(c, "id")
	if !ok || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "id 非法"})
		return
	}

	var row questionRow
	if err := QuestionDB.WithContext(c).Raw(sqlqueries.GetQuestionByIDQuery, id).Scan(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "查询失败: " + err.Error()})
		return
	}
	if row.ID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "题目不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"question": toQuestionJSON(row),
	})
}

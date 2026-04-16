package note

import "time"

// Note 是碎片笔记的领域模型
type Note struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"createdAt"`
	FilePath  string    `json:"filePath"`
}

// NoteSummary 用于列表展示的轻量摘要
type NoteSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"createdAt"`
	Preview   string    `json:"preview"`
}

// NoteFilter 笔记查询过滤条件
type NoteFilter struct {
	Date    *time.Time `json:"date,omitempty"`
	Month   string     `json:"month,omitempty"`
	Tags    []string   `json:"tags,omitempty"`
	Keyword string     `json:"keyword,omitempty"`
	Offset  int        `json:"offset"`
	Limit   int        `json:"limit"`
}

// CalendarHeatmapEntry 日历热力图单日数据
type CalendarHeatmapEntry struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// TagCount 标签及其出现计数
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

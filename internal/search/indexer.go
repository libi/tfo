package search

import (
	"context"
	"os"
	"time"

	"github.com/blevesearch/bleve/v2"
	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	_ "github.com/blevesearch/bleve/v2/analysis/lang/cjk"
	"github.com/blevesearch/bleve/v2/mapping"
)

// IndexDocument 索引中的文档结构
type IndexDocument struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"createdAt"`
}

// Indexer 管理搜索索引的创建、更新和删除
type Indexer interface {
	Open(indexPath string) error
	Close() error
	Index(doc *IndexDocument) error
	Remove(id string) error
	Rebuild(ctx context.Context, scanFn func(ctx context.Context, callback func(*IndexDocument) error) error) error
	NeedsRebuild() bool
	GetIndex() bleve.Index
}

// BleveIndexer Bleve 索引实现
type BleveIndexer struct {
	index     bleve.Index
	indexPath string
}

// NewBleveIndexer 创建一个新的 BleveIndexer
func NewBleveIndexer() *BleveIndexer {
	return &BleveIndexer{}
}

// Open 打开或创建 Bleve 索引
func (b *BleveIndexer) Open(indexPath string) error {
	b.indexPath = indexPath

	// 尝试打开已有索引
	idx, err := bleve.Open(indexPath)
	if err == nil {
		b.index = idx
		return nil
	}

	// 索引不存在则创建新索引
	idx, err = bleve.NewUsing(indexPath, buildIndexMapping(), "scorch", "scorch", nil)
	if err != nil {
		return err
	}
	b.index = idx
	return nil
}

// Close 安全关闭索引
func (b *BleveIndexer) Close() error {
	if b.index != nil {
		return b.index.Close()
	}
	return nil
}

// Index 添加或更新一个文档到索引
func (b *BleveIndexer) Index(doc *IndexDocument) error {
	if b.index == nil {
		return ErrIndexNotOpen
	}
	return b.index.Index(doc.ID, doc)
}

// Remove 从索引中删除一个文档
func (b *BleveIndexer) Remove(id string) error {
	if b.index == nil {
		return ErrIndexNotOpen
	}
	return b.index.Delete(id)
}

// Rebuild 全量重建索引
func (b *BleveIndexer) Rebuild(ctx context.Context, scanFn func(ctx context.Context, callback func(*IndexDocument) error) error) error {
	// 关闭当前索引
	if b.index != nil {
		_ = b.index.Close()
		b.index = nil
	}

	// 删除旧索引目录
	if err := os.RemoveAll(b.indexPath); err != nil {
		return err
	}

	// 创建新索引
	idx, err := bleve.NewUsing(b.indexPath, buildIndexMapping(), "scorch", "scorch", nil)
	if err != nil {
		return err
	}
	b.index = idx

	// 使用 batch 批量写入
	batch := b.index.NewBatch()
	count := 0

	err = scanFn(ctx, func(doc *IndexDocument) error {
		if err := batch.Index(doc.ID, doc); err != nil {
			return err
		}
		count++
		if count%100 == 0 {
			if err := b.index.Batch(batch); err != nil {
				return err
			}
			batch = b.index.NewBatch()
		}
		return nil
	})
	if err != nil {
		return err
	}

	if batch.Size() > 0 {
		return b.index.Batch(batch)
	}
	return nil
}

// NeedsRebuild 检查索引是否需要重建
func (b *BleveIndexer) NeedsRebuild() bool {
	if b.indexPath == "" {
		return true
	}
	_, err := os.Stat(b.indexPath)
	return os.IsNotExist(err)
}

// GetIndex 返回底层 bleve.Index 实例
func (b *BleveIndexer) GetIndex() bleve.Index {
	return b.index
}

// SetIndexPath 设置索引路径（在 Open 前使用，用于 NeedsRebuild 检查）
func (b *BleveIndexer) SetIndexPath(indexPath string) {
	b.indexPath = indexPath
}

// buildIndexMapping 构建 Bleve 索引映射
func buildIndexMapping() mapping.IndexMapping {
	noteMapping := bleve.NewDocumentMapping()

	// Title: CJK 分词 + 存储
	titleField := bleve.NewTextFieldMapping()
	titleField.Analyzer = AnalyzerName
	titleField.Store = true
	titleField.IncludeTermVectors = true
	noteMapping.AddFieldMappingsAt("title", titleField)

	// Content: CJK 分词, 存储（搜索结果需要返回原文用于高亮展示）
	contentField := bleve.NewTextFieldMapping()
	contentField.Analyzer = AnalyzerName
	contentField.Store = true
	contentField.IncludeTermVectors = true
	noteMapping.AddFieldMappingsAt("content", contentField)

	// Tags: keyword 不分词
	tagField := bleve.NewKeywordFieldMapping()
	noteMapping.AddFieldMappingsAt("tags", tagField)

	// CreatedAt: datetime
	dateField := bleve.NewDateTimeFieldMapping()
	noteMapping.AddFieldMappingsAt("createdAt", dateField)

	indexMapping := bleve.NewIndexMapping()

	// 注册带 output_unigram 的 CJK bigram filter，使单个汉字也能作为 unigram 被索引
	err := indexMapping.AddCustomTokenFilter(BigramUnigramName, map[string]interface{}{
		"type":           "cjk_bigram",
		"output_unigram": true,
	})
	if err != nil {
		panic("register tfo_cjk_bigram filter: " + err.Error())
	}

	// 注册自定义 analyzer：unicode tokenizer → cjk_width → lowercase → tfo_word_split → tfo_cjk_bigram
	err = indexMapping.AddCustomAnalyzer(AnalyzerName, map[string]interface{}{
		"type":      "custom",
		"tokenizer": "unicode",
		"token_filters": []interface{}{
			"cjk_width",
			"to_lower",
			WordSplitName,
			BigramUnigramName,
		},
	})
	if err != nil {
		panic("register tfo_cjk analyzer: " + err.Error())
	}

	indexMapping.DefaultMapping = noteMapping
	indexMapping.DefaultAnalyzer = AnalyzerName
	return indexMapping
}

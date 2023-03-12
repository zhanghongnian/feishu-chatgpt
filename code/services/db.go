package services

import (
	"context"
	"time"

	"github.com/didi/gendry/builder"
	"github.com/jmoiron/sqlx"
)

type FeiShuRecord struct {
	Id       int64     `db:"id"`
	OpenId   string    `db:"open_id"`
	Name     string    `db:"name"`
	Question string    `db:"question"`
	Answer   string    `db:"answer"`
	CreateAt time.Time `db:"create_at"`
}

var (
	tableTraverser = "feishu"
)

func InsertFeiShuRecord(ctx context.Context, db *sqlx.DB, record *FeiShuRecord) (err error) {
	data := []map[string]interface{}{{
		"open_id":   record.OpenId,
		"name":      record.Name,
		"question":  record.Question,
		"answer":    record.Answer,
		"create_at": record.CreateAt,
	}}

	cond, values, err := builder.BuildInsert(tableTraverser, data)
	if err != nil {
		return
	}

	_, err = db.ExecContext(ctx, cond, values...)
	if err != nil {
		return
	}

	return
}

package repository

import (
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	"habr-rss-bot/internal/domain"
)

type pgArticleRepository struct {
	db *sqlx.DB
}

func NewPostgresArticleRepository(db *sqlx.DB) domain.ArticleRepository {
	return &pgArticleRepository{db: db}
}

func (r *pgArticleRepository) Create(a *domain.Article) error {
	query := `
		INSERT INTO articles (guid, title, link, summary, image_url, pub_date, source_id)
		VALUES (:guid, :title, :link, :summary, :image_url, :pub_date, :source_id)
		ON CONFLICT (guid) DO NOTHING
	`
	_, err := r.db.NamedExec(query, a)
	return err
}

func (r *pgArticleRepository) GetLatest(limit int) ([]domain.Article, error) {
	var articles []domain.Article
	query := `SELECT * FROM articles ORDER BY pub_date DESC LIMIT $1`
	err := r.db.Select(&articles, query, limit)
	return articles, err
}

func (r *pgArticleRepository) GetByCategory(category string, limit int) ([]domain.Article, error) {
	var articles []domain.Article
	query := `
		SELECT a.* FROM articles a
		JOIN sources s ON a.source_id = s.id
		WHERE s.category = $1
		ORDER BY a.pub_date DESC
		LIMIT $2
	`
	err := r.db.Select(&articles, query, category, limit)
	return articles, err
}

func (r *pgArticleRepository) Exists(guid string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM articles WHERE guid = $1)`
	err := r.db.Get(&exists, query, guid)
	return exists, err
}

func (r *pgArticleRepository) DeleteOld(olderThan time.Time) error {
	query := `DELETE FROM articles WHERE created_at < $1`
	_, err := r.db.Exec(query, olderThan)
	return err
}

type pgSourceRepository struct {
	db *sqlx.DB
}

func NewPostgresSourceRepository(db *sqlx.DB) domain.SourceRepository {
	return &pgSourceRepository{db: db}
}

func (r *pgSourceRepository) GetAll() ([]domain.Source, error) {
	var sources []domain.Source
	query := `SELECT * FROM sources`
	err := r.db.Select(&sources, query)
	return sources, err
}

func (r *pgSourceRepository) GetByID(id int64) (*domain.Source, error) {
	var source domain.Source
	query := `SELECT * FROM sources WHERE id = $1`
	err := r.db.Get(&source, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &source, err
}

func (r *pgSourceRepository) Create(s *domain.Source) error {
	query := `INSERT INTO sources (name, url, category) VALUES (:name, :url, :category)`
	_, err := r.db.NamedExec(query, s)
	return err
}

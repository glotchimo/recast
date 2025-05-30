package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/glotchimo/recast/internal/models"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/graxinc/errutil"
)

type Database struct {
	l       *slog.Logger
	db      *sql.DB
	builder sq.StatementBuilderType
}

func NewDatabase(l *slog.Logger, databaseURL string) (*Database, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, errutil.With(err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	cache := sq.NewStmtCache(db)
	database := Database{l: l, db: db, builder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(cache)}

	if err := database.Migrate(databaseURL); err != nil {
		return nil, errutil.With(err)
	}

	return &database, nil
}

func (db *Database) Close() error {
	return db.db.Close()
}

func (db *Database) Migrate(databaseURL string) error {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return errutil.With(err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return errutil.With(err)
	}

	version, dirty, err := m.Version()
	if err != nil {
		return errutil.With(err)
	}

	db.l.Info("migrations applied", "version", version, "dirty", dirty)

	return nil
}

func (db *Database) Create(ctx context.Context, m models.Mappable) error {
	data := m.Map()
	data["created"] = time.Now().UTC()
	q := db.builder.
		Insert(string(m.Table())).
		SetMap(data)

	if _, err := q.ExecContext(ctx); err != nil {
		return errutil.With(err)
	}

	return nil
}

func (db *Database) Update(ctx context.Context, table models.Table, where sq.Eq, updates map[string]any) error {
	updates["updated"] = time.Now().UTC()
	q := db.builder.
		Update(string(table)).
		SetMap(updates).
		Where(where)
	if _, err := q.ExecContext(ctx); err != nil {
		return errutil.With(err)
	}

	return nil
}

func (db *Database) Delete(ctx context.Context, table models.Table, where sq.Eq) error {
	var hasDeletedColumn bool
	err := db.builder.
		Select("1").
		From("information_schema.columns").
		Where(sq.And{
			sq.Eq{"table_name": string(table)},
			sq.Eq{"column_name": "deleted"},
		}).
		QueryRowContext(ctx).
		Scan(&hasDeletedColumn)

	if err != nil && err != sql.ErrNoRows {
		return errutil.With(err)
	}

	if hasDeletedColumn {
		updates := map[string]any{
			"deleted": time.Now().UTC(),
			"updated": time.Now().UTC(),
		}

		q := db.builder.
			Update(string(table)).
			SetMap(updates).
			Where(where)

		if _, err := q.ExecContext(ctx); err != nil {
			return errutil.With(err)
		}
	} else {
		q := db.builder.
			Delete(string(table)).
			Where(where)

		if _, err := q.ExecContext(ctx); err != nil {
			return errutil.With(err)
		}
	}

	return nil
}

func (db *Database) Count(ctx context.Context, table models.Table, where sq.Eq) (int, error) {
	var count int

	q := db.builder.
		Select("COUNT(*)").
		From(string(table)).
		Where(where)

	if err := q.QueryRowContext(ctx).Scan(&count); err != nil {
		return count, errutil.With(err)
	}

	return count, nil
}

type Tx struct {
	tx      *sql.Tx
	builder sq.StatementBuilderType
	l       *slog.Logger
}

func (db *Database) BeginTx(ctx context.Context) (*Tx, error) {
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errutil.With(err)
	}

	return &Tx{
		tx:      tx,
		builder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(tx),
		l:       db.l,
	}, nil
}

func (tx *Tx) Commit() error {
	return tx.tx.Commit()
}

func (tx *Tx) Rollback() error {
	return tx.tx.Rollback()
}

func (tx *Tx) Create(ctx context.Context, m models.Mappable) error {
	data := m.Map()
	data["created"] = time.Now().UTC()

	q := tx.builder.
		Insert(string(m.Table())).
		SetMap(data)

	if _, err := q.ExecContext(ctx); err != nil {
		return errutil.With(err)
	}

	return nil
}

func (tx *Tx) Update(ctx context.Context, table models.Table, where sq.Eq, updates map[string]any) error {
	updates["updated"] = time.Now().UTC()

	q := tx.builder.
		Update(string(table)).
		SetMap(updates).
		Where(where)

	if _, err := q.ExecContext(ctx); err != nil {
		return errutil.With(err)
	}

	return nil
}

func (tx *Tx) Delete(ctx context.Context, table models.Table, where sq.Eq) error {
	var hasDeletedColumn bool
	err := tx.builder.
		Select("1").
		From("information_schema.columns").
		Where(sq.And{
			sq.Eq{"table_name": string(table)},
			sq.Eq{"column_name": "deleted"},
		}).
		QueryRowContext(ctx).
		Scan(&hasDeletedColumn)

	if err != nil && err != sql.ErrNoRows {
		return errutil.With(err)
	}

	if hasDeletedColumn {
		updates := map[string]any{
			"deleted": time.Now().UTC(),
			"updated": time.Now().UTC(),
		}

		q := tx.builder.
			Update(string(table)).
			SetMap(updates).
			Where(where)

		if _, err := q.ExecContext(ctx); err != nil {
			return errutil.With(err)
		}
	} else {
		q := tx.builder.
			Delete(string(table)).
			Where(where)

		if _, err := q.ExecContext(ctx); err != nil {
			return errutil.With(err)
		}
	}

	return nil
}

func (tx *Tx) Count(ctx context.Context, table models.Table, where sq.Eq) (int, error) {
	var count int

	q := tx.builder.
		Select("COUNT(*)").
		From(string(table)).
		Where(where)

	if err := q.QueryRowContext(ctx).Scan(&count); err != nil {
		return count, errutil.With(err)
	}

	return count, nil
}

func (db *Database) PutGuild(ctx context.Context, guild models.Guild) error {
	m := guild.Map()
	m["created"] = time.Now()
	q := db.builder.
		Insert(string(models.TableGuilds)).
		SetMap(m).
		Suffix(`ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`)
	if _, err := q.ExecContext(ctx); err != nil {
		return errutil.With(err)
	}

	return nil
}

func (db *Database) GetGuild(ctx context.Context, id string) (*models.Guild, error) {
	var g models.Guild
	var settingsRaw []byte

	q := db.builder.
		Select(
			"id",
			"name",
			"settings",
			"created",
			"updated",
			"deleted").
		From(string(models.TableGuilds)).
		Where(sq.Eq{"id": id})

	if err := q.QueryRowContext(ctx).Scan(
		&g.ID,
		&g.Name,
		&settingsRaw,
		&g.Created,
		&g.Updated,
		&g.Deleted,
	); err != nil {
		return nil, errutil.Wrap(err)
	}

	if err := json.Unmarshal(settingsRaw, &g.Settings); err != nil {
		return nil, errutil.With(err)
	}

	return &g, nil
}

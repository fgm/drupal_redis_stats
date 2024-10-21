package dock

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v4/stdlib"
)

type PgAdapter struct {
	conn *sql.DB
	pass string
}

func (pg *PgAdapter) CreatePhoneNumber(num string) (int, error) {
	if pg.conn == nil {
		return 0, errors.New("no connection")
	}
	const sq = `
INSERT INTO numbers(number) 
VALUES($1)
RETURNING id;
`
	// LastInsertId() is not supported by pgx, so we need this construct.
	var id int
	err := pg.conn.QueryRow(sq, num).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed inserting %s: %w", num, err)
	}
	return id, nil
}

func (pg *PgAdapter) RemovePhoneNumber(id int) error {
	if pg.conn == nil {
		return errors.New("no connection")
	}
	const sq = `
DELETE FROM numbers
WHERE id=$1;
`
	res, err := pg.conn.Exec(sq, id)
	if err != nil {
		return fmt.Errorf("failed deleting id %d: %w", id, err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed getting deleted rows for id %d: %w", id, err)
	}
	if ra != 1 {
		return fmt.Errorf("deleted %d rows for id %d, but expected 1", ra, id)
	}
	return nil
}

type Option func(adapter *PgAdapter)

func WithPassword(pass string) func(adapter *PgAdapter) {
	return func(adapter *PgAdapter) {
		adapter.pass = pass
	}
}

func NewAdapter(host, port, user, base string, options ...Option) (*PgAdapter, error) {
	pg := &PgAdapter{}
	for _, option := range options {
		option(pg)
	}
	c, err := sql.Open("pgx", fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, pg.pass, host, port, base))
	if err != nil {
		return nil, fmt.Errorf("opening DB: %w", err)
	}
	if err := c.Ping(); err != nil {
		log.Printf("Failed ping: %v", err)
		return nil, fmt.Errorf("pinging: %v", err)
	}

	pg.conn = c
	return pg, nil
}

func initTestAdapter(pg *PgAdapter) {
	if pg.conn == nil {
		panic(errors.New("connection not open"))
	}
	sq := `
CREATE TABLE public.numbers (
    id SERIAL NOT NULL,
    number character varying NOT NULL
);
ALTER TABLE ONLY public.numbers
    ADD CONSTRAINT numbers_pk PRIMARY KEY (id);
`
	res, err := pg.conn.Exec(sq)
	if err != nil {
		panic(err)
	}
	li, _ := res.LastInsertId()
	ra, _ := res.RowsAffected()
	log.Printf("Created table: %v %v", li, ra)
}

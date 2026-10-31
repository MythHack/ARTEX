package db

import (
	"database/sql"
	"encoding/json"
	"time"
)

// TaskIdentity is one test identity configured for a task. identity_id is
// carried on an intent payload as {"identity_id": n}; empty means anonymous.
type TaskIdentity struct {
	ID        int64             `json:"id"`
	TaskID    int64             `json:"task_id"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	Username  string            `json:"username"`
	Password  string            `json:"password"`
	Cookie    string            `json:"cookie"`
	Token     string            `json:"token"`
	Headers   map[string]string `json:"headers"`
	Notes     string            `json:"notes"`
	Enabled   bool              `json:"enabled"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

func (d *DB) ensureTaskIdentities() error {
	_, err := d.Exec(`CREATE TABLE IF NOT EXISTS task_identities (
	id BIGSERIAL PRIMARY KEY,
	task_id BIGINT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	kind TEXT NOT NULL CHECK (kind IN ('password','cookie','token','header')),
	username TEXT NOT NULL DEFAULT '',
	password TEXT NOT NULL DEFAULT '',
	cookie TEXT NOT NULL DEFAULT '',
	token TEXT NOT NULL DEFAULT '',
	headers JSONB NOT NULL DEFAULT '{}',
	notes TEXT NOT NULL DEFAULT '',
	enabled BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_task_identities_task ON task_identities(task_id);`)
	return err
}

func scanIdentity(sc interface{ Scan(...any) error }) (*TaskIdentity, error) {
	var it TaskIdentity
	var headers []byte
	err := sc.Scan(&it.ID, &it.TaskID, &it.Name, &it.Kind, &it.Username, &it.Password, &it.Cookie, &it.Token, &headers, &it.Notes, &it.Enabled, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return nil, err
	}
	it.Headers = map[string]string{}
	if len(headers) > 0 {
		_ = json.Unmarshal(headers, &it.Headers)
	}
	return &it, nil
}

const identityCols = `id,task_id,name,kind,username,password,cookie,token,headers,notes,enabled,created_at,updated_at`

func (d *DB) CreateTaskIdentity(it *TaskIdentity) (int64, error) {
	if err := d.ensureTaskIdentities(); err != nil {
		return 0, err
	}
	hb, _ := json.Marshal(it.Headers)
	if it.Headers == nil {
		hb = []byte(`{}`)
	}
	var id int64
	err := d.QueryRow(`INSERT INTO task_identities(task_id,name,kind,username,password,cookie,token,headers,notes,enabled)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, it.TaskID, it.Name, it.Kind, it.Username, it.Password, it.Cookie, it.Token, hb, it.Notes, it.Enabled).Scan(&id)
	return id, err
}

func (d *DB) ListTaskIdentities(taskID int64, onlyEnabled bool) ([]*TaskIdentity, error) {
	if err := d.ensureTaskIdentities(); err != nil {
		return nil, err
	}
	q := `SELECT ` + identityCols + ` FROM task_identities WHERE task_id=$1`
	if onlyEnabled {
		q += ` AND enabled`
	}
	q += ` ORDER BY id`
	rows, err := d.Query(q, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*TaskIdentity
	for rows.Next() {
		it, err := scanIdentity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (d *DB) UpdateTaskIdentity(it *TaskIdentity) error {
	if err := d.ensureTaskIdentities(); err != nil {
		return err
	}
	hb, _ := json.Marshal(it.Headers)
	if it.Headers == nil {
		hb = []byte(`{}`)
	}
	_, err := d.Exec(`UPDATE task_identities SET name=$2,kind=$3,username=$4,password=$5,cookie=$6,token=$7,headers=$8,notes=$9,enabled=$10,updated_at=now() WHERE id=$1`, it.ID, it.Name, it.Kind, it.Username, it.Password, it.Cookie, it.Token, hb, it.Notes, it.Enabled)
	return err
}

func (d *DB) DeleteTaskIdentity(id int64) error {
	_, err := d.Exec(`DELETE FROM task_identities WHERE id=$1`, id)
	return err
}

func (d *DB) GetTaskIdentity(id int64) (*TaskIdentity, error) {
	if err := d.ensureTaskIdentities(); err != nil {
		return nil, err
	}
	it, err := scanIdentity(d.QueryRow(`SELECT `+identityCols+` FROM task_identities WHERE id=$1`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return it, err
}

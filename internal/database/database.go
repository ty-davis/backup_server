package database

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

type DB struct {
	*sql.DB
}

type User struct {
	ID       int
	Username string
	Password string
	GroupIDs []int
}

type Group struct {
	ID   int
	Name string
}

type File struct {
	ID          int
	Name        string
	FilePath    string
	GroupID     int
	Description string
}

func InitDB(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := createTables(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL
	);

	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS user_groups (
		user_id INTEGER NOT NULL,
		group_id INTEGER NOT NULL,
		PRIMARY KEY (user_id, group_id),
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		file_path TEXT NOT NULL,
		group_id INTEGER NOT NULL,
		description TEXT,
		FOREIGN KEY (group_id) REFERENCES groups(id)
	);
	`

	_, err := db.Exec(schema)
	return err
}

func (db *DB) CreateGroup(name string) (int64, error) {
	result, err := db.Exec("INSERT INTO groups (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) GetGroupByID(groupID int) (*Group, error) {
	group := &Group{}
	err := db.QueryRow("SELECT id, name FROM groups WHERE id = ?", groupID).Scan(&group.ID, &group.Name)
	if err != nil {
		return nil, err
	}
	return group, nil
}

func (db *DB) UpdateGroup(groupID int, name string) error {
	_, err := db.Exec("UPDATE groups SET name = ? WHERE id = ?", name, groupID)
	return err
}

func (db *DB) DeleteGroup(groupID int) error {
	_, err := db.Exec("DELETE FROM groups WHERE id = ?", groupID)
	return err
}

func (db *DB) GetGroupMemberCount(groupID int) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM user_groups WHERE group_id = ?", groupID).Scan(&count)
	return count, err
}

func (db *DB) GetGroupFileCount(groupID int) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM files WHERE group_id = ?", groupID).Scan(&count)
	return count, err
}

func (db *DB) CreateUser(username, password string, groupIDs []int) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)",
		username, string(hash))
	if err != nil {
		return err
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	for _, groupID := range groupIDs {
		_, err = tx.Exec("INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)",
			userID, groupID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *DB) GetUserByUsername(username string) (*User, error) {
	user := &User{}
	err := db.QueryRow("SELECT id, username, password_hash FROM users WHERE username = ?",
		username).Scan(&user.ID, &user.Username, &user.Password)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query("SELECT group_id FROM user_groups WHERE user_id = ?", user.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groupIDs []int
	for rows.Next() {
		var groupID int
		if err := rows.Scan(&groupID); err != nil {
			return nil, err
		}
		groupIDs = append(groupIDs, groupID)
	}

	user.GroupIDs = groupIDs
	return user, rows.Err()
}

func (db *DB) ValidateUser(username, password string) (*User, error) {
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	return user, nil
}

func (db *DB) AddFile(name, filePath string, groupID int, description string) error {
	_, err := db.Exec("INSERT INTO files (name, file_path, group_id, description) VALUES (?, ?, ?, ?)",
		name, filePath, groupID, description)
	return err
}

func (db *DB) GetFilesByGroupID(groupID int) ([]File, error) {
	rows, err := db.Query("SELECT id, name, file_path, group_id, description FROM files WHERE group_id = ?", groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.Name, &f.FilePath, &f.GroupID, &f.Description); err != nil {
			return nil, err
		}
		files = append(files, f)
	}

	return files, rows.Err()
}

func (db *DB) GetFilesByGroupIDs(groupIDs []int) ([]File, error) {
	if len(groupIDs) == 0 {
		return []File{}, nil
	}

	query := "SELECT DISTINCT id, name, file_path, group_id, description FROM files WHERE group_id IN ("
	args := make([]interface{}, len(groupIDs))
	for i, id := range groupIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ")"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.Name, &f.FilePath, &f.GroupID, &f.Description); err != nil {
			return nil, err
		}
		files = append(files, f)
	}

	return files, rows.Err()
}

func (db *DB) GetFileByID(fileID int) (*File, error) {
	file := &File{}
	err := db.QueryRow("SELECT id, name, file_path, group_id, description FROM files WHERE id = ?",
		fileID).Scan(&file.ID, &file.Name, &file.FilePath, &file.GroupID, &file.Description)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (db *DB) UserHasAccessToGroup(userID, groupID int) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM user_groups WHERE user_id = ? AND group_id = ?",
		userID, groupID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (db *DB) GetAllFiles() ([]File, error) {
	rows, err := db.Query("SELECT id, name, file_path, group_id, description FROM files ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.Name, &f.FilePath, &f.GroupID, &f.Description); err != nil {
			return nil, err
		}
		files = append(files, f)
	}

	return files, rows.Err()
}

func (db *DB) GetAllGroups() ([]Group, error) {
	rows, err := db.Query("SELECT id, name FROM groups ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}

	return groups, rows.Err()
}

func (db *DB) UpdateFile(fileID int, name, filePath string, groupID int, description string) error {
	_, err := db.Exec("UPDATE files SET name = ?, file_path = ?, group_id = ?, description = ? WHERE id = ?",
		name, filePath, groupID, description, fileID)
	return err
}

func (db *DB) DeleteFile(fileID int) error {
	_, err := db.Exec("DELETE FROM files WHERE id = ?", fileID)
	return err
}

func (db *DB) GetAllUsers() ([]User, error) {
	rows, err := db.Query("SELECT id, username, password_hash FROM users ORDER BY username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Password); err != nil {
			return nil, err
		}

		groupRows, err := db.Query("SELECT group_id FROM user_groups WHERE user_id = ?", u.ID)
		if err != nil {
			return nil, err
		}

		var groupIDs []int
		for groupRows.Next() {
			var groupID int
			if err := groupRows.Scan(&groupID); err != nil {
				groupRows.Close()
				return nil, err
			}
			groupIDs = append(groupIDs, groupID)
		}
		groupRows.Close()

		u.GroupIDs = groupIDs
		users = append(users, u)
	}

	return users, rows.Err()
}

func (db *DB) GetUserByID(userID int) (*User, error) {
	user := &User{}
	err := db.QueryRow("SELECT id, username, password_hash FROM users WHERE id = ?",
		userID).Scan(&user.ID, &user.Username, &user.Password)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query("SELECT group_id FROM user_groups WHERE user_id = ?", user.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groupIDs []int
	for rows.Next() {
		var groupID int
		if err := rows.Scan(&groupID); err != nil {
			return nil, err
		}
		groupIDs = append(groupIDs, groupID)
	}

	user.GroupIDs = groupIDs
	return user, rows.Err()
}

func (db *DB) UpdateUser(userID int, username string, groupIDs []int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE users SET username = ? WHERE id = ?", username, userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM user_groups WHERE user_id = ?", userID)
	if err != nil {
		return err
	}

	for _, groupID := range groupIDs {
		_, err = tx.Exec("INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)", userID, groupID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *DB) UpdateUserPassword(userID int, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(hash), userID)
	return err
}

func (db *DB) DeleteUser(userID int) error {
	_, err := db.Exec("DELETE FROM users WHERE id = ?", userID)
	return err
}

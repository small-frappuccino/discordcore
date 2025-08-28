package example

// import (
//     "database/sql"
//     "time"

//     _ "github.com/mattn/go-sqlite3"
// )

// // SQLiteDatabase implementa Database usando SQLite
// type SQLiteDatabase struct {
//     db *sql.DB
// }

// // NewSQLiteDatabase cria uma nova instância
// func NewSQLiteDatabase(dbPath string) (*SQLiteDatabase, error) {
//     db, err := sql.Open("sqlite3", dbPath)
//     if err != nil {
//         return nil, err
//     }

//     // Criar tabela se não existir
//     _, err = db.Exec(`
//         CREATE TABLE IF NOT EXISTS avatar_records (
//             user_id TEXT,
//             username TEXT,
//             avatar TEXT,
//             avatar_url TEXT,
//             changed_at DATETIME,
//             guild_id TEXT
//         )
//     `)
//     if err != nil {
//         return nil, err
//     }

//     return &SQLiteDatabase{db: db}, nil
// }

// // SaveAvatarRecord salva um registro
// func (db *SQLiteDatabase) SaveAvatarRecord(record AvatarRecord) error {
//     _, err := db.db.Exec(`
//         INSERT INTO avatar_records (user_id, username, avatar, avatar_url, changed_at, guild_id)
//         VALUES (?, ?, ?, ?, ?, ?)
//     `, record.UserID, record.Username, record.Avatar, record.AvatarURL, record.ChangedAt, record.GuildID)
//     return err
// }

// // GetAvatarHistory retorna histórico de um usuário
// func (db *SQLiteDatabase) GetAvatarHistory(userID string) ([]AvatarRecord, error) {
//     rows, err := db.db.Query(`
//         SELECT user_id, username, avatar, avatar_url, changed_at, guild_id
//         FROM avatar_records
//         WHERE user_id = ?
//         ORDER BY changed_at DESC
//     `, userID)
//     if err != nil {
//         return nil, err
//     }
//     defer rows.Close()

//     var records []AvatarRecord
//     for rows.Next() {
//         var record AvatarRecord
//         err := rows.Scan(&record.UserID, &record.Username, &record.Avatar, &record.AvatarURL, &record.ChangedAt, &record.GuildID)
//         if err != nil {
//             return nil, err
//         }
//         records = append(records, record)
//     }
//     return records, nil
// }

// // GetSuspiciousChanges retorna mudanças suspeitas nas últimas horas
// func (db *SQLiteDatabase) GetSuspiciousChanges(hours int) ([]AvatarRecord, error) {
//     cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
//     rows, err := db.db.Query(`
//         SELECT user_id, username, avatar, avatar_url, changed_at, guild_id
//         FROM avatar_records
//         WHERE changed_at > ?
//     `, cutoff)
//     if err != nil {
//         return nil, err
//     }
//     defer rows.Close()

//     var records []AvatarRecord
//     for rows.Next() {
//         var record AvatarRecord
//         err := rows.Scan(&record.UserID, &record.Username, &record.Avatar, &record.AvatarURL, &record.ChangedAt, &record.GuildID)
//         if err != nil {
//             return nil, err
//         }
//         records = append(records, record)
//     }
//     return records, nil
// }

// // Close fecha a conexão
// func (db *SQLiteDatabase) Close() error {
//     return db.db.Close()
// }

package example

// import (
// 	"encoding/json"
// 	"os"
// 	"time"
// )

// // JSONDatabase implementa Database usando arquivos JSON
// type JSONDatabase struct {
// 	filePath string
// 	data     map[string][]AvatarRecord
// }

// // NewJSONDatabase cria uma nova instância
// func NewJSONDatabase(filePath string) *JSONDatabase {
// 	db := &JSONDatabase{
// 		filePath: filePath,
// 		data:     make(map[string][]AvatarRecord),
// 	}
// 	db.loadFromFile()
// 	return db
// }

// // SaveAvatarRecord salva um registro
// func (db *JSONDatabase) SaveAvatarRecord(record AvatarRecord) error {
// 	db.data[record.UserID] = append(db.data[record.UserID], record)
// 	return db.saveToFile()
// }

// // GetAvatarHistory retorna histórico de um usuário
// func (db *JSONDatabase) GetAvatarHistory(userID string) ([]AvatarRecord, error) {
// 	return db.data[userID], nil
// }

// // GetSuspiciousChanges retorna mudanças suspeitas nas últimas horas
// func (db *JSONDatabase) GetSuspiciousChanges(hours int) ([]AvatarRecord, error) {
// 	var suspicious []AvatarRecord
// 	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)

// 	for _, records := range db.data {
// 		for _, record := range records {
// 			if record.ChangedAt.After(cutoff) {
// 				// Lógica para detectar suspeitas (exemplo simplificado)
// 				suspicious = append(suspicious, record)
// 			}
// 		}
// 	}
// 	return suspicious, nil
// }

// // Métodos auxiliares para carregar/salvar arquivo
// func (db *JSONDatabase) loadFromFile() error {
// 	if _, err := os.Stat(db.filePath); os.IsNotExist(err) {
// 		return nil // Arquivo não existe, começa vazio
// 	}
// 	file, err := os.Open(db.filePath)
// 	if err != nil {
// 		return err
// 	}
// 	defer file.Close()
// 	return json.NewDecoder(file).Decode(&db.data)
// }

// func (db *JSONDatabase) saveToFile() error {
// 	file, err := os.Create(db.filePath)
// 	if err != nil {
// 		return err
// 	}
// 	defer file.Close()
// 	return json.NewEncoder(file).Encode(db.data)
// }

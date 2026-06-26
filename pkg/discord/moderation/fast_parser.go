package moderation

import (
	"github.com/buger/jsonparser"
)

// extractStringFast navega pelos bytes do JSON puro e extrai o valor.
// Funciona em tempo O(N) (sendo N a profundidade da chave) sem criar structs na Heap.
func extractStringFast(payload []byte, keys ...string) string {
	val, err := jsonparser.GetString(payload, keys...)
	if err != nil {
		return "" // Retorna vazio se a chave não existir (seguro para campos opcionais)
	}
	return val
}

// extractCommandNameFast é um helper direto para ir buscar "data.name".
func extractCommandNameFast(payload []byte) string {
	return extractStringFast(payload, "data", "name")
}

// extractOptionString varre o array "options" do Discord de forma ultrarrápida.
// O Discord aninha os argumentos dos Slash Commands como um array de objetos:
// "options": [{"name": "target_id", "value": "123"}, {"name": "reason", "value": "spam"}]
func extractOptionString(payload []byte, targetOptionName string) string {
	var result string

	// ArrayEach iterará sobre os bytes do array diretamente (Zero-Allocation).
	// Em vez de decodificar o array todo, paramos quando encontramos a opção.
	jsonparser.ArrayEach(payload, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		name, _ := jsonparser.GetString(value, "name")
		if name == targetOptionName {
			result, _ = jsonparser.GetString(value, "value")
		}
	}, "data", "options")

	return result
}

// extractOptionInt extrai um inteiro de forma segura do array "options".
func extractOptionInt(payload []byte, targetOptionName string) int {
	var result int64

	jsonparser.ArrayEach(payload, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		name, _ := jsonparser.GetString(value, "name")
		if name == targetOptionName {
			result, _ = jsonparser.GetInt(value, "value")
		}
	}, "data", "options")

	return int(result)
}

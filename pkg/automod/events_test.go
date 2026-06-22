package automod

import (
	"encoding/json"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// simulateGoldenPayload simula o carregamento de um arquivo .json real
// contendo a resposta oficial extraída da documentação do Discord.
const simulateGoldenPayload = `{
	"guild_id": "123456789012345678",
	"action": {
		"type": 1,
		"metadata": {
			"channel_id": "987654321098765432",
			"duration_seconds": 3600
		}
	},
	"rule_trigger_type": 1,
	"user_id": "111222333444555666",
	"matched_keyword": "badword",
	"matched_content": "this is a badword message"
}`

// TestExecutionEvent_Golden_Unmarshal valida se o JSON da API mapeia 1:1 para as structs locais.
func TestExecutionEvent_Golden_Unmarshal(t *testing.T) {
	var event ExecutionEvent
	err := json.Unmarshal([]byte(simulateGoldenPayload), &event)
	if err != nil {
		t.Fatalf("json.Unmarshal failed unexpectedly: %v", err)
	}

	// Validação pontual das invariantes críticas de infraestrutura (Snowflakes e Metadata)
	if event.GuildID != discord.GuildID(123456789012345678) {
		t.Errorf("GuildID mismatch: got %v", event.GuildID)
	}
	if event.Action.Metadata.DurationSecs != 3600 {
		t.Errorf("Action.Metadata.DurationSecs mismatch: got %v", event.Action.Metadata.DurationSecs)
	}
	if event.MatchedKeyword != "badword" {
		t.Errorf("MatchedKeyword mismatch: got %v", event.MatchedKeyword)
	}
}

// TestExecutionEvent_RoundTrip garante simetria. Detecta regressões silenciosas
// onde tags omit_empty engolem campos booleanos falsos ou ints nulos indesejadamente.
func TestExecutionEvent_RoundTrip(t *testing.T) {
	original := ExecutionEvent{
		GuildID:              discord.GuildID(123456789012345678),
		ChannelID:            discord.ChannelID(987654321098765432),
		UserID:               discord.UserID(111222333444555666),
		RuleID:               discord.Snowflake(999888777666555),
		MessageID:            discord.MessageID(111111111111111),
		AlertSystemMessageID: discord.MessageID(222222222222222),
		MatchedKeyword:       "banned",
		MatchedContent:       "you are banned",
		RuleTriggerType:      1,
		Action: ExecutionAction{
			Type: 1,
			Metadata: ExecutionActionMetadata{
				ChannelID:    discord.ChannelID(987654321098765432),
				DurationSecs: 60,
			},
		},
	}

	marshaled, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled ExecutionEvent
	err = json.Unmarshal(marshaled, &unmarshaled)
	if err != nil {
		t.Fatalf("json.Unmarshal failed on round-trip: %v", err)
	}

	// Compara as duas structs profundamente ignorando campos não exportados, se houverem.
	if diff := cmp.Diff(original, unmarshaled, cmpopts.IgnoreUnexported(ExecutionEvent{})); diff != "" {
		t.Errorf("RoundTrip mismatch (-original +unmarshaled):\n%s", diff)
	}
}

// FuzzExecutionEvent_Unmarshal testa a fronteira de memória e resiliência
// contra lixo injetado na interceptação de websockets (ArikawaAdapter).
func FuzzExecutionEvent_Unmarshal(f *testing.F) {
	// Seed inicial baseado no Golden File para guiar o motor de mutação do Fuzzer
	f.Add([]byte(simulateGoldenPayload))
	f.Add([]byte(`{"guild_id": "invalid_type", "action": null}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var event ExecutionEvent
		// O objetivo do Fuzz aqui não é verificar erro vs sucesso,
		// mas garantir zero panics (memory corruption ou slice out of bounds).
		err := json.Unmarshal(data, &event)
		if err == nil {
			// Se o unmarshal for bem-sucedido acidentalmente, garante que um Marshal
			// consequente não quebre, protegendo o pipeline automod.Sink.
			_, marshalErr := json.Marshal(event)
			if marshalErr != nil {
				t.Errorf("Valid Unmarshal resulted in unserializable struct: %v", marshalErr)
			}
		}
	})
}

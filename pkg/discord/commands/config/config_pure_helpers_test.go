package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestFormatQOTDSchedule(t *testing.T) {
	t.Parallel()

	intPtr := func(value int) *int {
		return &value
	}

	tests := []struct {
		name     string
		schedule files.QOTDPublishScheduleConfig
		want     string
	}{
		{
			name:     "zero schedule renders dash",
			schedule: files.QOTDPublishScheduleConfig{},
			want:     "—",
		},
		{
			name: "partial hour renders placeholder minute",
			schedule: files.QOTDPublishScheduleConfig{
				HourUTC: intPtr(12),
			},
			want: "12:--",
		},
		{
			name: "partial minute renders placeholder hour",
			schedule: files.QOTDPublishScheduleConfig{
				MinuteUTC: intPtr(43),
			},
			want: "--:43",
		},
		{
			name: "complete schedule renders utc time",
			schedule: files.QOTDPublishScheduleConfig{
				HourUTC:   intPtr(12),
				MinuteUTC: intPtr(43),
			},
			want: "12:43",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatQOTDSchedule(tt.schedule); got != tt.want {
				t.Fatalf("formatQOTDSchedule() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTranslateQOTDConfigError(t *testing.T) {
	t.Parallel()

	errOther := errors.New("other failure")
	tests := []struct {
		name               string
		input              error
		wantErr            error
		wantCommandMessage string
		wantEphemeral      bool
	}{
		{
			name:  "nil stays nil",
			input: nil,
		},
		{
			name:    "non qotd error passes through",
			input:   errOther,
			wantErr: errOther,
		},
		{
			name:               "generic invalid qotd input becomes command error",
			input:              fmt.Errorf("%w: %s", files.ErrInvalidQOTDInput, ""),
			wantCommandMessage: "Invalid QOTD configuration",
			wantEphemeral:      true,
		},
		{
			name:               "missing schedule validation maps to command guidance",
			input:              fmt.Errorf("%w: %s", files.ErrInvalidQOTDInput, "schedule.hour_utc and schedule.minute_utc are required when enabled"),
			wantCommandMessage: "Set the QOTD publish hour and minute before enabling publishing",
			wantEphemeral:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translateQOTDConfigError(tt.input)

			if tt.wantErr != nil {
				if got != tt.wantErr {
					t.Fatalf("translateQOTDConfigError() returned %v, want original %v", got, tt.wantErr)
				}
				return
			}

			if tt.wantCommandMessage == "" {
				if got != nil {
					t.Fatalf("translateQOTDConfigError() = %v, want nil", got)
				}
				return
			}

			var commandErr *core.CommandError
			if !errors.As(got, &commandErr) {
				t.Fatalf("translateQOTDConfigError() returned %T, want *core.CommandError", got)
			}
			if commandErr.Message != tt.wantCommandMessage || commandErr.Ephemeral != tt.wantEphemeral {
				t.Fatalf("translateQOTDConfigError() = %+v, want message=%q ephemeral=%v", commandErr, tt.wantCommandMessage, tt.wantEphemeral)
			}
		})
	}
}
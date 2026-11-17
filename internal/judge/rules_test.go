package judge

import (
	"context"
	"testing"
)

func TestRulesJudge_Evaluate(t *testing.T) {
	j := NewRulesJudge()
	ctx := context.Background()

	tests := []struct {
		name        string
		content     Content
		wantVerdict Verdict
		wantReason  string
	}{
		{
			name:        "normal game review passes",
			content:     Content{Title: "Obra maestra", Body: "La historia y los combates contra jefes son increíbles."},
			wantVerdict: Approve,
			wantReason:  "",
		},
		{
			name:        "rejects suspicious url",
			content:     Content{Title: "Monedas gratis", Body: "Entra a https://free-coins.xyz/promo para ganar"},
			wantVerdict: Reject,
			wantReason:  "spam_links",
		},
		{
			name:        "rejects repetitive character spam",
			content:     Content{Title: "Juego", Body: "buenooooooooooooooo jajajajajaja"},
			wantVerdict: Reject,
			wantReason:  "spam_repetitive",
		},
		{
			name:        "rejects leetspeak slur",
			content:     Content{Title: "Malísimo", Body: "El autor de arriba es un m4r1c0n total"},
			wantVerdict: Reject,
			wantReason:  "hate_or_harassment",
		},
		{
			name:        "rejects self harm encouragement",
			content:     Content{Title: "Basura", Body: "Si te gusta este juego mejor suicidate"},
			wantVerdict: Reject,
			wantReason:  "self_harm_or_threat",
		},
		{
			name:        "rejects suicidal ideation and self harm intent",
			content:     Content{Title: "me quiero suicidar", Body: "este juego me dio depresion... de vdd ojala morir, un arma en la cabeza y ya... osea lit me quiero suicidar, osea lit,"},
			wantVerdict: Reject,
			wantReason:  "self_harm_or_threat",
		},
		{
			name:        "flags excessive shouting as doubt",
			content:     Content{Title: "NO LO COMPREN", Body: "ESTE JUEGO ES UNA ESTAFA TOTAL NO VALE NADA DE DINERO QUE HORROR"},
			wantVerdict: Doubt,
			wantReason:  "excessive_caps",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := j.Evaluate(ctx, tc.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Verdict != tc.wantVerdict {
				t.Errorf("Verdict = %q, want %q", got.Verdict, tc.wantVerdict)
			}
			if got.Reason != tc.wantReason {
				t.Errorf("Reason = %q, want %q", got.Reason, tc.wantReason)
			}
		})
	}
}

package domain

// Arquivo descartável: prova que o CI reprova domain importando adapters (#4).
// Este PR não deve ser mergeado.

import "github.com/lastrohq/lastro/internal/adapters/jobs"

// RodarPeriodico só existe para usar o import proibido.
var RodarPeriodico = jobs.Periodico

package app

// Arquivo descartável: prova que a regra de app (modo deny, só produção) dispara (#4).

import "github.com/lastrohq/lastro/internal/adapters/jobs"

// RodarPeriodico só existe para usar o import proibido.
var RodarPeriodico = jobs.Periodico

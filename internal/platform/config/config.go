// Package config lê a configuração comum a todos os serviços (12-factor).
// Configuração específica de um serviço (ex.: USERS_URL no orders) é lida
// pelo próprio serviço com Require/Getenv.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr        string // ex.: ":8080"
	DBDriver        string // "postgres" ou "memory"
	DatabaseURL     string // usado quando DBDriver == "postgres"
	ProblemTypeBase string // prefixo do campo "type" do Problem Details; vazio => about:blank
	OIDC            OIDC
}

// OIDC configura como o serviço valida tokens e, se ele chama outros
// serviços, como obtém os seus próprios.
type OIDC struct {
	IssuerURL    string // claim "iss" esperado, ex.: http://localhost:8280/realms/app
	DiscoveryURL string // opcional: endereço interno do Keycloak (ex.: dentro do compose)
	Audience     string // client ID DESTE serviço no Keycloak
	ClientID     string // credenciais deste serviço para chamar outros (opcional)
	ClientSecret string
}

// TokenURL é o endpoint de token do Keycloak, pelo endereço que o serviço alcança.
func (o OIDC) TokenURL() string {
	base := o.DiscoveryURL
	if base == "" {
		base = o.IssuerURL
	}
	return strings.TrimSuffix(base, "/") + "/protocol/openid-connect/token"
}

// Load lê a configuração comum. audience é o client ID padrão do serviço.
func Load(audience string) (Config, error) {
	cfg := Config{
		HTTPAddr:        Getenv("HTTP_ADDR", ":8080"),
		DBDriver:        Getenv("DB_DRIVER", "postgres"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		ProblemTypeBase: os.Getenv("PROBLEM_TYPE_BASE"),
		OIDC: OIDC{
			IssuerURL:    os.Getenv("OIDC_ISSUER_URL"),
			DiscoveryURL: os.Getenv("OIDC_DISCOVERY_URL"),
			Audience:     Getenv("OIDC_AUDIENCE", audience),
			ClientID:     Getenv("OIDC_CLIENT_ID", audience),
			ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		},
	}

	var errs []error
	switch cfg.DBDriver {
	case "memory":
	case "postgres":
		if cfg.DatabaseURL == "" {
			errs = append(errs, errors.New("DATABASE_URL é obrigatório com DB_DRIVER=postgres"))
		}
	default:
		errs = append(errs, fmt.Errorf("DB_DRIVER inválido %q (use postgres ou memory)", cfg.DBDriver))
	}
	// Não existe "modo sem autenticação": é o tipo de flag que acaba ligada em produção.
	if cfg.OIDC.IssuerURL == "" {
		errs = append(errs, errors.New("OIDC_ISSUER_URL é obrigatório"))
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

// Require lê variáveis obrigatórias, acumulando todas as que faltam.
func Require(keys ...string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	var missing []string
	for _, k := range keys {
		v := os.Getenv(k)
		if v == "" {
			missing = append(missing, k)
		}
		out[k] = v
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("config: obrigatório: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

func Getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bensolo-io/jwt-kit/internal/idp"
	"github.com/brianvoe/gofakeit"
	"github.com/golang-jwt/jwt"
	"github.com/spf13/cobra"
)

type Config struct {
	Claims      []string
	Scopes      []string
	Audiences   []string
	Exp         string
	Sub         string
	claimsMap   jwt.MapClaims
	expDuration time.Duration
	PrettyPrint bool
}

var config Config = Config{claimsMap: make(map[string]interface{})}

var rootCmd = &cobra.Command{
	Use:   "jwt-kit",
	Short: "jwt-kit - a simple CLI to generate JWTs using a development IDP",
	Long: fmt.Sprintf(`Jwt-kit contains an embedded keypair used to sign jwts.

Public JWKS url: %s

Issuer name: %s
`, idp.JWKSUrl, idp.Issuer),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var errs []string

		for _, c := range config.Claims {
			parts := strings.Split(c, "=")
			if len(parts) != 2 {
				errs = append(errs, fmt.Sprintf("arg '%s' must be in format key=value", c))
				continue
			}
			config.claimsMap[parts[0]] = parts[1]
		}

		var err error
		config.expDuration, err = time.ParseDuration(config.Exp)
		if err != nil {
			errs = append(errs, fmt.Sprintf("invalid time duration '%s': %s", config.Exp, err))
		}

		if len(errs) > 0 {
			return fmt.Errorf("claims validation errors: %s", strings.Join(errs, "; "))
		}
		gofakeit.Seed(time.Now().UnixNano())
		config.claimsMap["beer_of_the_day"] = gofakeit.BeerName()
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		theJwt := getUnsignedJwt()

		tokenString, err := theJwt.SignedString(idp.GetRSAPrivateKey())
		if err != nil {
			return err
		}

		if config.PrettyPrint {
			theToken, err := parseSignedJwtString(tokenString)
			if err != nil {
				return err
			}
			formatted, err := json.MarshalIndent(theToken, "", "  ")
			if err != nil {
				return err
			}
			fmt.Printf("\n%s\n", formatted)
		} else {
			fmt.Println(tokenString)
		}

		return nil
	},
}

func Execute() {
	rootCmd.Flags().StringArrayVarP(&config.Claims, "claims", "c", []string{}, "add jwt claims")
	rootCmd.Flags().StringArrayVarP(&config.Scopes, "scopes", "s", []string{}, "add jwt scopes")
	rootCmd.Flags().StringArrayVarP(&config.Audiences, "audiences", "a", []string{"https://fake-resource.solo.io"}, "jwt audience")
	rootCmd.Flags().StringVarP(&config.Exp, "expires-in", "e", "8766h", "expires duration (uses https://pkg.go.dev/time#ParseDuration)")
	rootCmd.Flags().StringVarP(&config.Sub, "subject", "u", "glooey@solo.io", "jwt subject")
	rootCmd.Flags().BoolVarP(&config.PrettyPrint, "pretty-print", "p", false, "pretty print the token")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}

func getUnsignedJwt() *jwt.Token {
	token := jwt.New(jwt.SigningMethodRS256)
	token.Header["kid"] = idp.Kid

	now := time.Now().UTC()

	config.claimsMap["exp"] = now.Add(config.expDuration).Unix()
	config.claimsMap["iss"] = idp.Issuer
	config.claimsMap["aud"] = config.Audiences
	config.claimsMap["sub"] = config.Sub
	config.claimsMap["scopes"] = config.Scopes
	token.Claims = config.claimsMap

	return token
}

func parseSignedJwtString(token string) (*jwt.Token, error) {
	t, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected method: %s", t.Header["alg"])
		}
		return idp.GetRSAPublicKey(), nil
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}
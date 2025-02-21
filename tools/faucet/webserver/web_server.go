package webserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/gin-gonic/gin"
	"github.com/obscuronet/go-obscuro/tools/faucet/faucet"
)

type WebServer struct {
	engine      *gin.Engine
	faucet      *faucet.Faucet
	bindAddress string
	server      *http.Server
}

type requestAddr struct {
	Address string `json:"address" binding:"required"`
}

func NewWebServer(faucetServer *faucet.Faucet, bindAddress string, jwtSecret []byte) *WebServer {
	r := gin.New()
	gin.SetMode(gin.ReleaseMode)

	// todo move this declaration out of this scope
	parseFunding := func(c *gin.Context) {
		tokenReq := c.Params.ByName("token")
		var token string

		// check the token request type
		switch tokenReq {
		case faucet.OBXNativeToken:
			token = faucet.OBXNativeToken
		case faucet.WrappedOBX:
			token = faucet.WrappedOBX
		case faucet.WrappedEth:
			token = faucet.WrappedEth
		case faucet.WrappedUSDC:
			token = faucet.WrappedUSDC
		default:
			errorHandler(c, fmt.Errorf("token not recognized: %s", tokenReq), faucetServer.Logger)
			return
		}

		// make sure there's an address
		var req requestAddr
		if err := c.Bind(&req); err != nil {
			errorHandler(c, fmt.Errorf("unable to parse request: %w", err), faucetServer.Logger)
			return
		}

		// make sure the address is valid
		if !common.IsHexAddress(req.Address) {
			errorHandler(c, fmt.Errorf("unexpected address %s", req.Address), faucetServer.Logger)
			return
		}

		amount := int64(100)

		// fund the address
		addr := common.HexToAddress(req.Address)
		if err := faucetServer.Fund(&addr, token, amount); err != nil {
			errorHandler(c, fmt.Errorf("unable to fund request %w", err), faucetServer.Logger)
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}

	jwtTokenCheck := func(c *gin.Context) {
		jwtToken, err := extractBearerToken(c.GetHeader("Authorization"))
		if err != nil {
			errorHandler(c, err, faucetServer.Logger)
			return
		}

		_, err = faucet.ValidateToken(jwtToken, jwtSecret)
		if err != nil {
			errorHandler(c, err, faucetServer.Logger)
			return
		}

		c.Next()
	}
	// authed endpoint
	r.POST("/auth/fund/:token", jwtTokenCheck, parseFunding)

	r.POST("/fund/:token", parseFunding)

	return &WebServer{
		engine:      r,
		faucet:      faucetServer,
		bindAddress: bindAddress,
	}
}

func (w *WebServer) Start() error {
	w.server = &http.Server{
		Addr:              w.bindAddress,
		Handler:           w.engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	return nil
}

func (w *WebServer) Stop() error {
	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return w.server.Shutdown(ctx)
}

func errorHandler(c *gin.Context, err error, logger log.Logger) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
		"error": err.Error(),
	})
	logger.Error(err.Error())
}

func extractBearerToken(header string) (string, error) {
	if header == "" {
		return "", fmt.Errorf("bad header value given")
	}

	jwtToken := strings.Split(header, " ")
	if len(jwtToken) != 2 {
		return "", fmt.Errorf("incorrectly formatted authorization header")
	}

	return jwtToken[1], nil
}

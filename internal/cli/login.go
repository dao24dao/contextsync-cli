package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"contextsync/internal/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to your ContextSync account",
	Long: `Login to your ContextSync account using email verification.

You will receive a 6-digit verification code via email.
After verification, you can use all ContextSync features.`,
	Run: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from your ContextSync account",
	Run:   runLogout,
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}

func runLogin(cmd *cobra.Command, args []string) {
	// Check if already logged in
	if config.IsLoggedIn() {
		email := config.GetAccountEmail()
		fmt.Printf("Already logged in as %s\n", email)
		fmt.Println("Use 'contextsync logout' to logout first.")
		return
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("       ContextSync Login")
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println()

	// Get email
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	if email == "" {
		fmt.Println("Error: Email is required")
		os.Exit(1)
	}

	// Send verification code
	fmt.Println()
	fmt.Printf("Sending verification code to %s...\n", email)

	if err := sendVerificationCode(email); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✉️Verification code sent!")
	fmt.Println("   Please check your inbox (and spam folder)")
	fmt.Println()

	// Get verification code
	fmt.Print("Enter verification code: ")
	code, _ := reader.ReadString('\n')
	code = strings.TrimSpace(code)

	if len(code) != 6 {
		fmt.Println("Error: Verification code must be 6 digits")
		os.Exit(1)
	}

	// Verify code
	fmt.Println()
	fmt.Println("Verifying...")

	account, err := verifyCode(email, code)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Save account info
	config.SetAccount(account.Account.ID, account.Account.Email, account.Token)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("✅ Login successful!")
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("Account: %s\n", account.Account.Email)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Run 'contextsync init' to set up your tools")
	fmt.Println("  2. Run 'contextsync status' to view your account")
	fmt.Println()
}

func runLogout(cmd *cobra.Command, args []string) {
	if !config.IsLoggedIn() {
		fmt.Println("Not logged in")
		return
	}

	email := config.GetAccountEmail()
	config.ClearAccount()

	fmt.Printf("Logged out from %s\n", email)
}

type sendCodeRequest struct {
	Email string `json:"email"`
}

type verifyCodeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type verifyCodeResponse struct {
	Success bool        `json:"success"`
	Token   string      `json:"token"`
	Account AccountInfo `json:"account"`
	Error   string      `json:"error"`
}

type AccountInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func sendVerificationCode(email string) error {
	serverURL := config.GetServerURL()

	body := sendCodeRequest{Email: email}
	jsonBody, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST",
		serverURL+"/api/v1/auth/send-code",
		strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var result struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("failed to send code: %s", result.Error)
	}

	return nil
}

func verifyCode(email, code string) (*verifyCodeResponse, error) {
	serverURL := config.GetServerURL()

	body := verifyCodeRequest{
		Email: email,
		Code:  code,
	}
	jsonBody, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST",
		serverURL+"/api/v1/auth/verify",
		strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	var result verifyCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid response from server")
	}

	if !result.Success {
		return nil, fmt.Errorf("%s", result.Error)
	}

	return &result, nil
}

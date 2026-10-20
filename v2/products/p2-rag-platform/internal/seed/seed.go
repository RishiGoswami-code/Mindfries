// Package seed loads a small knowledge base and evaluation set so the RAG
// platform answers sensibly the moment it starts.
package seed

import "github.com/mindfries/products/p2-rag-platform/internal/rag"

// Docs is the sample knowledge base.
func Docs() []rag.Document {
	return []rag.Document{
		{ID: "login", Title: "Logging in and passwords", Text: "If you cannot log in, use the forgot password link on the login page to reset your password and regain access to your account. Passwords must be at least twelve characters."},
		{ID: "twofa", Title: "Two-factor authentication", Text: "Enable two-factor authentication from your security settings using an authenticator app. Two-factor adds a second step at login to protect your account from unauthorized access."},
		{ID: "billing", Title: "Billing and invoices", Text: "You can download invoices and receipts from the billing page in your account settings. Invoices are issued monthly for your current subscription plan."},
		{ID: "refund", Title: "Refund policy", Text: "Refunds for the current billing period can be requested from the billing page within thirty days of payment. Refunds are returned to the original payment method."},
		{ID: "export", Title: "Exporting your data", Text: "Export your workspace data as a CSV file from the admin data export panel. Large exports are prepared in the background and emailed to you when ready."},
	}
}

// Cases is the default evaluation set.
func Cases() []rag.Case {
	return []rag.Case{
		{Query: "how do I reset my password", RelevantDocIDs: []string{"login"}, WantSubstr: "password"},
		{Query: "turn on two factor authentication", RelevantDocIDs: []string{"twofa"}, WantSubstr: "authentication"},
		{Query: "where are my invoices", RelevantDocIDs: []string{"billing"}, WantSubstr: "invoices"},
		{Query: "can I get a refund", RelevantDocIDs: []string{"refund"}, WantSubstr: "refund"},
		{Query: "export my data to csv", RelevantDocIDs: []string{"export"}, WantSubstr: "export"},
	}
}

// Load ingests the sample docs into an engine.
func Load(e *rag.Engine) {
	for _, d := range Docs() {
		e.Ingest(d)
	}
}

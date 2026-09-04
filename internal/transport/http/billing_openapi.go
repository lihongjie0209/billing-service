package httptransport

// createPlanDocs godoc
// @Summary Create a billing plan
// @Tags plans
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body createPlanRequest true "Plan"
// @Success 200 {object} Response{body=PlanBody}
// @Router /api/v1/plans/create [post]
func createPlanDocs() {}

// updatePlanDocs godoc
// @Summary Update a billing plan with optimistic locking
// @Tags plans
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body updatePlanRequest true "Plan and current version"
// @Success 200 {object} Response{body=PlanBody}
// @Router /api/v1/plans/update [post]
func updatePlanDocs() {}

// getPlanDocs godoc
// @Summary Get a plan and its usage prices
// @Tags plans
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body getPlanRequest true "Plan identity"
// @Success 200 {object} Response{body=PlanDetailBody}
// @Router /api/v1/plans/get [post]
func getPlanDocs() {}

// listPlansDocs godoc
// @Summary Search billing plans
// @Tags plans
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body listPlansRequest true "Filters and pagination"
// @Success 200 {object} Response{body=PlanPageBody}
// @Router /api/v1/plans/list [post]
func listPlansDocs() {}

// listAvailablePlansDocs godoc
// @Summary List active plans available for subscription
// @Tags subscriptions
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body listAvailablePlansRequest true "Search and pagination"
// @Success 200 {object} Response{body=PlanPageBody}
// @Router /api/v1/subscriptions/plans/list [post]
func listAvailablePlansDocs() {} //nolint:unused // Swagger discovers annotation holders during generation.

// upsertUsagePriceDocs godoc
// @Summary Create or update a metered usage price
// @Tags plans
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body upsertUsagePriceRequest true "Usage price"
// @Success 200 {object} Response{body=UsagePriceBody}
// @Router /api/v1/plans/usage-prices/upsert [post]
func upsertUsagePriceDocs() {}

// deleteUsagePriceDocs godoc
// @Summary Delete a usage price with optimistic locking
// @Tags plans
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body deleteUsagePriceRequest true "Usage price and parent plan versions"
// @Success 200 {object} Response
// @Router /api/v1/plans/usage-prices/delete [post]
func deleteUsagePriceDocs() {}

// createSubscriptionDocs godoc
// @Summary Subscribe a tenant to a plan
// @Tags subscriptions
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body createSubscriptionRequest true "Subscription"
// @Success 200 {object} Response{body=SubscriptionBody}
// @Router /api/v1/subscriptions/create [post]
func createSubscriptionDocs() {}

// changeSubscriptionDocs godoc
// @Summary Change a subscription immediately or next period
// @Tags subscriptions
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body changeSubscriptionRequest true "Plan change"
// @Success 200 {object} Response{body=SubscriptionBody}
// @Router /api/v1/subscriptions/change [post]
func changeSubscriptionDocs() {}

// cancelSubscriptionDocs godoc
// @Summary Cancel a subscription immediately or at period end
// @Tags subscriptions
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body cancelSubscriptionRequest true "Cancellation"
// @Success 200 {object} Response{body=SubscriptionBody}
// @Router /api/v1/subscriptions/cancel [post]
func cancelSubscriptionDocs() {}

// getSubscriptionDocs godoc
// @Summary Get a tenant subscription and plan
// @Tags subscriptions
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body getSubscriptionRequest true "Subscription identity"
// @Success 200 {object} Response{body=SubscriptionDetailBody}
// @Router /api/v1/subscriptions/get [post]
func getSubscriptionDocs() {}

// listSubscriptionsDocs godoc
// @Summary List tenant subscriptions
// @Tags subscriptions
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body listSubscriptionsRequest true "Filters and pagination"
// @Success 200 {object} Response{body=SubscriptionPageBody}
// @Router /api/v1/subscriptions/list [post]
func listSubscriptionsDocs() {}

// previewInvoiceDocs godoc
// @Summary Preview an invoice using metering totals
// @Tags invoices
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body invoicePeriodRequest true "Invoice period"
// @Success 200 {object} Response{body=InvoiceDetailBody}
// @Router /api/v1/invoices/preview [post]
func previewInvoiceDocs() {}

// generateInvoiceDocs godoc
// @Summary Generate an idempotent invoice
// @Tags invoices
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body invoicePeriodRequest true "Invoice period and idempotency key"
// @Success 200 {object} Response{body=GenerateInvoiceBody}
// @Router /api/v1/invoices/generate [post]
func generateInvoiceDocs() {}

// finalizeInvoiceDocs godoc
// @Summary Finalize a draft invoice
// @Tags invoices
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body finalizeInvoiceRequest true "Invoice and due date"
// @Success 200 {object} Response{body=InvoiceBody}
// @Router /api/v1/invoices/finalize [post]
func finalizeInvoiceDocs() {}

// voidInvoiceDocs godoc
// @Summary Void an unpaid invoice
// @Tags invoices
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body voidInvoiceRequest true "Invoice and reason"
// @Success 200 {object} Response{body=InvoiceBody}
// @Router /api/v1/invoices/void [post]
func voidInvoiceDocs() {}

// getInvoiceDocs godoc
// @Summary Get an invoice and line items
// @Tags invoices
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body getInvoiceRequest true "Invoice identity"
// @Success 200 {object} Response{body=InvoiceDetailBody}
// @Router /api/v1/invoices/get [post]
func getInvoiceDocs() {}

// listInvoicesDocs godoc
// @Summary Search tenant invoices
// @Tags invoices
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body listInvoicesRequest true "Filters and pagination"
// @Success 200 {object} Response{body=InvoicePageBody}
// @Router /api/v1/invoices/list [post]
func listInvoicesDocs() {}

// createPaymentDocs godoc
// @Summary Create an idempotent payment attempt
// @Tags payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body createPaymentRequest true "Payment"
// @Success 200 {object} Response{body=CreatePaymentAttemptBody}
// @Router /api/v1/payments/create-attempt [post]
func createPaymentDocs() {}

// getPaymentDocs godoc
// @Summary Get an application payment attempt
// @Tags payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body getPaymentRequest true "Payment identity"
// @Success 200 {object} Response{body=PaymentAttemptBody}
// @Router /api/v1/payments/get [post]
func getPaymentDocs() {}

// listPaymentsDocs godoc
// @Summary List application payment attempts
// @Tags payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body listPaymentsRequest true "Filters and pagination"
// @Success 200 {object} Response{body=PaymentAttemptPageBody}
// @Router /api/v1/payments/list [post]
func listPaymentsDocs() {}

// applyPaymentDocs godoc
// @Summary Apply an idempotent payment-provider result
// @Tags payments
// @Accept json
// @Produce json
// @Security PSK
// @Param request body applyPaymentRequest true "Provider result"
// @Success 200 {object} Response{body=ApplyPaymentResultBody}
// @Router /api/v1/payments/apply-result [post]
func applyPaymentDocs() {}

// recordRefundDocs godoc
// @Summary Record an idempotent refund
// @Tags payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body recordRefundRequest true "Refund"
// @Success 200 {object} Response{body=RecordRefundBody}
// @Router /api/v1/payments/refunds/record [post]
func recordRefundDocs() {}

// listRefundsDocs godoc
// @Summary List application refunds
// @Tags payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body listPaymentsRequest true "Filters and pagination"
// @Success 200 {object} Response{body=RefundPageBody}
// @Router /api/v1/payments/refunds/list [post]
func listRefundsDocs() {}

var BillingOpenAPIOperations = []func(){
	createPlanDocs, updatePlanDocs, getPlanDocs, listPlansDocs,
	upsertUsagePriceDocs, deleteUsagePriceDocs,
	createSubscriptionDocs, changeSubscriptionDocs, cancelSubscriptionDocs, getSubscriptionDocs, listSubscriptionsDocs,
	previewInvoiceDocs, generateInvoiceDocs, finalizeInvoiceDocs, voidInvoiceDocs, getInvoiceDocs, listInvoicesDocs,
	createPaymentDocs, getPaymentDocs, listPaymentsDocs, applyPaymentDocs, recordRefundDocs, listRefundsDocs,
}

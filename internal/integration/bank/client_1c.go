package bank

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// OneCClient implements BankService for 1C bank exchange format (1CClientBankExchange).
type OneCClient struct{}

func NewOneCClient() *OneCClient { return &OneCClient{} }

// ImportStatement parses a 1C bank statement text file and returns an ImportResult.
func (c *OneCClient) ImportStatement(_ context.Context, _ uuid.UUID, data []byte) (*ImportResult, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	var totalDebit, totalCredit int64
	txCount := 0
	period := ""
	inSection := ""

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "ДатаНачала=") {
			period = strings.TrimPrefix(line, "ДатаНачала=")
		}

		if strings.HasPrefix(line, "СекцияДокумент=") {
			inSection = strings.TrimPrefix(line, "СекцияДокумент=")
			txCount++
		}
		if line == "КонецДокумента" {
			inSection = ""
		}

		if inSection != "" && strings.HasPrefix(line, "Сумма=") {
			sumStr := strings.TrimPrefix(line, "Сумма=")
			sumStr = strings.ReplaceAll(sumStr, ",", ".")
			amount, err := strconv.ParseFloat(sumStr, 64)
			if err == nil {
				kopecks := int64(amount * 100)
				if inSection == "Платёжное поручение" {
					totalDebit += kopecks
				} else {
					totalCredit += kopecks
				}
			}
		}
	}

	return &ImportResult{
		TransactionsImported: txCount,
		TotalDebit:           totalDebit,
		TotalCredit:          totalCredit,
		Period:               period,
		ImportedAt:           time.Now(),
	}, nil
}

// ExportPaymentOrders generates a 1C bank exchange format file from payment orders.
func (c *OneCClient) ExportPaymentOrders(_ context.Context, _ uuid.UUID, orders []PaymentOrder) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("1CClientBankExchange\n")
	buf.WriteString("ВерсияФормата=1.03\n")
	buf.WriteString("Кодировка=UTF-8\n")
	buf.WriteString("Отправитель=BizEngine\n")
	buf.WriteString("Получатель=Банк\n")
	buf.WriteString(fmt.Sprintf("ДатаСоздания=%s\n", time.Now().Format("02.01.2006")))
	buf.WriteString(fmt.Sprintf("ВремяСоздания=%s\n", time.Now().Format("15:04:05")))

	for _, order := range orders {
		buf.WriteString("СекцияДокумент=Платёжное поручение\n")
		buf.WriteString(fmt.Sprintf("Номер=%s\n", order.Number))
		buf.WriteString(fmt.Sprintf("Дата=%s\n", order.Date))
		buf.WriteString(fmt.Sprintf("Сумма=%.2f\n", float64(order.Amount)/100))
		buf.WriteString(fmt.Sprintf("ПлательщикИНН=%s\n", order.PayerINN))
		buf.WriteString(fmt.Sprintf("ПлательщикРасчСчет=%s\n", order.PayerAccount))
		buf.WriteString(fmt.Sprintf("ПолучательИНН=%s\n", order.ReceiverINN))
		buf.WriteString(fmt.Sprintf("ПолучательРасчСчет=%s\n", order.ReceiverAccount))
		buf.WriteString(fmt.Sprintf("НазначениеПлатежа=%s\n", order.Purpose))
		buf.WriteString("КонецДокумента\n")
	}

	buf.WriteString("КонецФайла\n")
	return buf.Bytes(), nil
}

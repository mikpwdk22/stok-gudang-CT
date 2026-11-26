package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"ctstock/backend/internal/config"
	"ctstock/backend/internal/database"
	"github.com/xuri/excelize/v2"
)

// ParsedProduct represents a product extracted from Excel
type ParsedProduct struct {
	SheetName     string
	RowIndex      int
	Name          string
	Brand         *string
	StockUnit     *string
	Price1        *float64
	RawPrice1     *string
	Price2        *float64
	RawPrice2     *string
	PriceCash     *float64
	RawPriceCash  *string
	PricingBasis  *string
	CurrentStock  *float64
	StockCounted  bool
	NeedsReview   bool
	ReviewReasons []string
}

// SkippedRow logs any non-product rows with reasons
type SkippedRow struct {
	SheetName string
	RowIndex  int
	Content   string
	Reason    string
}

// OpeningStockItem represents an entry from "Stok Barang Depan"
type OpeningStockItem struct {
	RowIndex    int
	Name        string
	Brand       string
	RawStock    string
	ParsedQty   *float64
	Unit        string
	IsAmbiguous bool
	Status      string // "RELIABLE", "AMBIGUOUS", "UNCOUNTED"
}

// BulkMaterialItem represents an entry from "Pasir& Split"
type BulkMaterialItem struct {
	Name     string
	RawPrice string
	Price    *float64
	Note     string
}

func main() {
	dryRun := flag.Bool("dry-run", true, "Perform dry-run validation without writing to database")
	execute := flag.Bool("execute", false, "Write parsed records into database")
	excelPathFlag := flag.String("file", "", "Path to the Excel file")
	flag.Parse()

	// If execute flag is passed, disable dryRun
	if *execute {
		*dryRun = false
	}

	excelPath := *excelPathFlag
	if excelPath == "" {
		// Detect default locations
		candidates := []string{
			`C:\Users\LENOVO\OneDrive\Documents\Cahaya Terang agustus.xlsx`,
			`C:\Users\LENOVO\OneDrive\Documents\Cahaya Terang.xlsx`,
			`..\Cahaya Terang agustus.xlsx`,
			`Cahaya Terang agustus.xlsx`,
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				excelPath = c
				break
			}
		}
	}

	if excelPath == "" {
		log.Fatalf("Excel file not found. Please specify using -file=\"<path-to-excel>\"")
	}

	absPath, _ := filepath.Abs(excelPath)
	fmt.Printf("========================================================\n")
	fmt.Printf("  CTStock Excel Importer\n")
	fmt.Printf("  Target File: %s\n", absPath)
	if *dryRun {
		fmt.Printf("  Mode:        [DRY RUN - PREVIEW ONLY, NO DB WRITES]\n")
	} else {
		fmt.Printf("  Mode:        [EXECUTE - WRITING TO POSTGRESQL]\n")
	}
	fmt.Printf("========================================================\n\n")

	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		log.Fatalf("Failed to open excel file: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	fmt.Printf("Found %d sheet(s) in workbook: %s\n\n", len(sheets), strings.Join(sheets, ", "))

	var allProducts []ParsedProduct
	var skippedRows []SkippedRow
	var ambiguousPrices []ParsedProduct
	var unverifiedUnits []ParsedProduct
	var potentialDuplicates []ParsedProduct
	seenKeys := make(map[string]int)

	var openingStockItems []OpeningStockItem
	var bulkItems []BulkMaterialItem

	// 1. Process Alphabetical Product Sheets
	for _, sheetName := range sheets {
		cleanName := strings.TrimSpace(sheetName)
		if cleanName == "Stok Barang Depan" || strings.Contains(cleanName, "Stok") {
			openingStockItems = parseStokBarangDepan(f, sheetName)
			continue
		}
		if cleanName == "Pasir& Split" || strings.Contains(cleanName, "Pasir") {
			bulkItems = parsePasirSplit(f, sheetName)
			continue
		}

		// Standard Alphabetical Sheets (A~B, C~D, etc.)
		rows, err := f.GetRows(sheetName)
		if err != nil {
			log.Printf("Warning: Failed to read rows in sheet %s: %v\n", sheetName, err)
			continue
		}

		for rowIdx, row := range rows {
			rowNum := rowIdx + 1
			if len(row) == 0 {
				continue
			}

			col0 := ""
			if len(row) > 0 {
				col0 = strings.TrimSpace(row[0])
			}
			col1 := ""
			if len(row) > 1 {
				col1 = strings.TrimSpace(row[1])
			}

			// Check for completely blank product name
			if col0 == "" {
				// Check if there is content in other columns that might be floating
				hasAny := false
				for _, cell := range row {
					if strings.TrimSpace(cell) != "" {
						hasAny = true
						break
					}
				}
				if hasAny {
					skippedRows = append(skippedRows, SkippedRow{
						SheetName: sheetName,
						RowIndex:  rowNum,
						Content:   strings.Join(row, " | "),
						Reason:    "Empty product name (Nama Barang kosong)",
					})
				}
				continue
			}

			// Check for Header repetition
			if strings.EqualFold(col0, "Nama Barang") {
				skippedRows = append(skippedRows, SkippedRow{
					SheetName: sheetName,
					RowIndex:  rowNum,
					Content:   strings.Join(row, " | "),
					Reason:    "Header row repeat (Nama Barang)",
				})
				continue
			}

			// Check for Discount/Formula header rows like ",List, Jual -26.5%"
			if strings.HasPrefix(col0, "List") || strings.HasPrefix(col0, ",List") || strings.Contains(col0, "Jual -26") {
				skippedRows = append(skippedRows, SkippedRow{
					SheetName: sheetName,
					RowIndex:  rowNum,
					Content:   strings.Join(row, " | "),
					Reason:    "Formula/List reference row",
				})
				continue
			}

			// Check for known technical reference sections (e.g. Rumus Siku Lobang)
			if strings.Contains(col0, "Rumus Siku") || strings.HasPrefix(col0, "(0,9 ~ 1 mm)") || strings.HasPrefix(col0, "(1,2 ~ 1,3 mm)") ||
				strings.HasPrefix(col0, "(1,4 ~ 1,5 mm)") || strings.HasPrefix(col0, "(1,6 mm)") || strings.HasPrefix(col0, "(1,7 mm)") ||
				strings.HasPrefix(col0, "(1,8 mm)") || strings.HasPrefix(col0, "(2 mm") {
				skippedRows = append(skippedRows, SkippedRow{
					SheetName: sheetName,
					RowIndex:  rowNum,
					Content:   strings.Join(row, " | "),
					Reason:    "Reference calculation section (Rumus)",
				})
				continue
			}

			// Valid Product Entry Candidate
			p := ParsedProduct{
				SheetName: sheetName,
				RowIndex:  rowNum,
				Name:      col0,
			}

			if col1 != "" {
				p.Brand = &col1
			}

			// Extract Unit from Product Name if present
			unit := extractUnitFromName(col0)
			if unit != "" {
				p.StockUnit = &unit
			} else {
				p.NeedsReview = true
				p.ReviewReasons = append(p.ReviewReasons, "Unit not identified (Satuan belum ditentukan)")
				unverifiedUnits = append(unverifiedUnits, p)
			}

			// Extract Prices (Cols 2, 3, 4: Harga 1, Harga 2, Cash)
			if len(row) > 2 && strings.TrimSpace(row[2]) != "" {
				raw := strings.TrimSpace(row[2])
				p.RawPrice1 = &raw
				val, basis := parsePriceWithBasis(raw)
				p.Price1 = val
				if basis != "" && p.PricingBasis == nil {
					p.PricingBasis = &basis
				}
				if basis != "" {
					ambiguousPrices = append(ambiguousPrices, p)
				}
			}

			if len(row) > 3 && strings.TrimSpace(row[3]) != "" {
				raw := strings.TrimSpace(row[3])
				p.RawPrice2 = &raw
				val, basis := parsePriceWithBasis(raw)
				p.Price2 = val
				if basis != "" && p.PricingBasis == nil {
					p.PricingBasis = &basis
				}
			}

			if len(row) > 4 && strings.TrimSpace(row[4]) != "" {
				raw := strings.TrimSpace(row[4])
				p.RawPriceCash = &raw
				val, basis := parsePriceWithBasis(raw)
				p.PriceCash = val
				if basis != "" && p.PricingBasis == nil {
					p.PricingBasis = &basis
				}
			}

			// Stock starts as NULL (Belum dihitung)
			p.CurrentStock = nil
			p.StockCounted = false

			// Check for duplicate variant in sheets
			brandKey := ""
			if p.Brand != nil {
				brandKey = *p.Brand
			}
			uniqueKey := strings.ToLower(p.Name + "|||" + brandKey)
			if prevRow, exists := seenKeys[uniqueKey]; exists {
				p.ReviewReasons = append(p.ReviewReasons, fmt.Sprintf("Duplicate variant (same name & brand as row %d)", prevRow))
				potentialDuplicates = append(potentialDuplicates, p)
			} else {
				seenKeys[uniqueKey] = rowNum
			}

			allProducts = append(allProducts, p)
		}
	}

	// 2. Display Dry-Run Diagnostic Report
	fmt.Printf("--------------------------------------------------------\n")
	fmt.Printf("1. PARSING SUMMARY\n")
	fmt.Printf("--------------------------------------------------------\n")
	fmt.Printf("Total Products Extracted:          %d items\n", len(allProducts))
	fmt.Printf("Non-product Rows Skipped:          %d rows\n", len(skippedRows))
	fmt.Printf("Prices with Measurement Suffixes:  %d items (e.g. /Kg, /psg, /mm)\n", len(ambiguousPrices))
	fmt.Printf("Products Requiring Unit Review:    %d items\n", len(unverifiedUnits))
	fmt.Printf("Potential Duplicate Variants:      %d items\n", len(potentialDuplicates))
	fmt.Printf("Opening Stock Sheet Items:         %d items (from Stok Barang Depan)\n", len(openingStockItems))
	fmt.Printf("Bulk Materials Items:              %d items (from Pasir& Split)\n\n", len(bulkItems))

	// Sample of prices with annotations
	if len(ambiguousPrices) > 0 {
		fmt.Printf("--------------------------------------------------------\n")
		fmt.Printf("2. SAMPLE: PRICES WITH MEASUREMENT SUFFIXES (Normalized & Raw Preserved)\n")
		fmt.Printf("--------------------------------------------------------\n")
		limit := 8
		if len(ambiguousPrices) < limit {
			limit = len(ambiguousPrices)
		}
		for i := 0; i < limit; i++ {
			p := ambiguousPrices[i]
			raw := ""
			if p.RawPrice1 != nil {
				raw = *p.RawPrice1
			}
			basis := ""
			if p.PricingBasis != nil {
				basis = *p.PricingBasis
			}
			fmt.Printf("  • [%s - %s] Raw: '%s' -> Numeric: %.2f | Basis: '%s'\n",
				p.Name, strVal(p.Brand, "Generic"), raw, floatVal(p.Price1), basis)
		}
		if len(ambiguousPrices) > limit {
			fmt.Printf("  ... and %d more items with preserved basis.\n", len(ambiguousPrices)-limit)
		}
		fmt.Println()
	}

	// Opening Stock Sheet Breakdown
	fmt.Printf("--------------------------------------------------------\n")
	fmt.Printf("3. 'STOK BARANG DEPAN' (22 ENTRIES REVIEW)\n")
	fmt.Printf("--------------------------------------------------------\n")
	for _, item := range openingStockItems {
		switch item.Status {
		case "RELIABLE":
			fmt.Printf("  [RELIABLE]     %s (%s): %.2f %s\n", item.Name, item.Brand, *item.ParsedQty, item.Unit)
		case "AMBIGUOUS":
			fmt.Printf("  [AMBIGUOUS]    %s (%s): Raw '%s' -> Marked for manual review!\n", item.Name, item.Brand, item.RawStock)
		case "UNCOUNTED":
			fmt.Printf("  [BELUM HITUNG] %s (%s): Empty count in sheet -> Kept as NULL\n", item.Name, item.Brand)
		}
	}
	fmt.Println()

	// Bulk Materials Sheet Breakdown
	if len(bulkItems) > 0 {
		fmt.Printf("--------------------------------------------------------\n")
		fmt.Printf("4. 'PASIR& SPLIT' BULK MATERIALS SHEET (Isolated Review)\n")
		fmt.Printf("--------------------------------------------------------\n")
		for _, b := range bulkItems {
			fmt.Printf("  • %s: Rp %s (%s)\n", b.Name, b.RawPrice, b.Note)
		}
		fmt.Println()
	}

	// If dry run, stop here and guide the user
	if *dryRun {
		fmt.Printf("========================================================\n")
		fmt.Printf("  DRY-RUN VALIDATION COMPLETE!\n")
		fmt.Printf("  No data was written to PostgreSQL.\n")
		fmt.Printf("  To execute the import, run with: -execute\n")
		fmt.Printf("========================================================\n")
		return
	}

	// 3. EXECUTE IMPORT INTO POSTGRESQL
	fmt.Printf("Connecting to PostgreSQL to execute import...\n")
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	// Run migrations to ensure all tables exist
	migrator := database.NewMigrationRunner(db.Pool, "migrations")
	if err := migrator.Up(ctx); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Fetch Admin user for transaction creator
	var adminUserID int
	err = tx.QueryRow(ctx, "SELECT id FROM users ORDER BY id ASC LIMIT 1").Scan(&adminUserID)
	if err != nil {
		log.Fatalf("No user found to associate opening stock transactions. Please run seeder first: %v", err)
	}

	insertedProducts := 0
	updatedProducts := 0

	insertProductSQL := `
		INSERT INTO products (
			name, brand, stock_unit,
			price_1, price_2, price_cash,
			raw_price_1, raw_price_2, raw_price_cash,
			pricing_basis, current_stock, is_stock_counted,
			min_stock_alert, is_active
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
		ON CONFLICT (name, COALESCE(brand, '')) DO UPDATE SET
			price_1 = EXCLUDED.price_1,
			price_2 = EXCLUDED.price_2,
			price_cash = EXCLUDED.price_cash,
			raw_price_1 = EXCLUDED.raw_price_1,
			raw_price_2 = EXCLUDED.raw_price_2,
			raw_price_cash = EXCLUDED.raw_price_cash,
			pricing_basis = EXCLUDED.pricing_basis,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, (xmax = 0) AS is_inserted;
	`

	productMap := make(map[string]int)

	for _, p := range allProducts {
		var productID int
		var isInserted bool

		err := tx.QueryRow(ctx, insertProductSQL,
			p.Name, p.Brand, p.StockUnit,
			p.Price1, p.Price2, p.PriceCash,
			p.RawPrice1, p.RawPrice2, p.RawPriceCash,
			p.PricingBasis, p.CurrentStock, p.StockCounted,
			5.0, true,
		).Scan(&productID, &isInserted)

		if err != nil {
			log.Fatalf("Failed to insert/update product '%s': %v", p.Name, err)
		}

		brandStr := ""
		if p.Brand != nil {
			brandStr = *p.Brand
		}
		mapKey := strings.ToLower(p.Name + "|||" + brandStr)
		productMap[mapKey] = productID

		if isInserted {
			insertedProducts++
		} else {
			updatedProducts++
		}
	}

	fmt.Printf("Products: %d inserted, %d updated.\n", insertedProducts, updatedProducts)

	// Process opening stock from Stok Barang Depan
	openingBalancesCreated := 0
	for _, item := range openingStockItems {
		if item.Status != "RELIABLE" || item.ParsedQty == nil {
			continue
		}

		// Find matching product
		lookupKey := strings.ToLower(item.Name + "|||" + item.Brand)
		prodID, found := productMap[lookupKey]
		if !found {
			// Try fuzzy/relaxed matching if exact match not found
			var matchedID int
			err := tx.QueryRow(ctx, `
				SELECT id FROM products
				WHERE name ILIKE $1 AND brand ILIKE $2
				LIMIT 1
			`, "%"+item.Name+"%", "%"+item.Brand+"%").Scan(&matchedID)
			if err == nil {
				prodID = matchedID
				found = true
			}
		}

		if !found {
			fmt.Printf("Note: Opening stock item '%s (%s)' could not be matched with catalog.\n", item.Name, item.Brand)
			continue
		}

		// Idempotency check: Check if opening_stock transaction already exists for this product
		var exists bool
		checkTxSQL := `
			SELECT EXISTS(
				SELECT 1 FROM stock_transactions st
				JOIN stock_transaction_items sti ON st.id = sti.transaction_id
				WHERE st.transaction_type = 'opening_stock' AND sti.product_id = $1
			)
		`
		_ = tx.QueryRow(ctx, checkTxSQL, prodID).Scan(&exists)
		if exists {
			continue
		}

		// Update product current_stock and mark as counted
		_, err := tx.Exec(ctx, `
			UPDATE products SET
				current_stock = $1,
				stock_unit = COALESCE(stock_unit, $2),
				is_stock_counted = true,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = $3
		`, *item.ParsedQty, item.Unit, prodID)
		if err != nil {
			log.Fatalf("Failed to update opening stock on product %d: %v", prodID, err)
		}

		// Create stock_transactions record
		var txID int
		err = tx.QueryRow(ctx, `
			INSERT INTO stock_transactions (transaction_type, user_id, notes, transaction_date)
			VALUES ('opening_stock', $1, 'Stok Awal dari Excel Stok Barang Depan', CURRENT_TIMESTAMP)
			RETURNING id
		`, adminUserID).Scan(&txID)
		if err != nil {
			log.Fatalf("Failed to insert opening transaction: %v", err)
		}

		// Create stock_transaction_items record
		_, err = tx.Exec(ctx, `
			INSERT INTO stock_transaction_items (transaction_id, product_id, quantity, unit_snapshot)
			VALUES ($1, $2, $3, $4)
		`, txID, prodID, *item.ParsedQty, item.Unit)
		if err != nil {
			log.Fatalf("Failed to insert opening transaction line item: %v", err)
		}

		openingBalancesCreated++
	}

	// Commit Transaction
	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("Failed to commit import transaction: %v", err)
	}

	fmt.Printf("\n========================================================\n")
	fmt.Printf("  IMPORT SUCCESSFUL!\n")
	fmt.Printf("  Total Catalog Products in DB:   %d\n", insertedProducts+updatedProducts)
	fmt.Printf("  Opening Balance Transactions:   %d\n", openingBalancesCreated)
	fmt.Printf("========================================================\n")
}

// Helpers

func extractUnitFromName(name string) string {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "mtr") || strings.Contains(lower, "meter") {
		if strings.Contains(lower, "rol") {
			return "rol"
		}
		return "meter"
	}
	if strings.Contains(lower, "kg") {
		return "kg"
	}
	if strings.Contains(lower, "pcs") || strings.Contains(lower, "pc)") || strings.Contains(lower, "pc )") {
		return "pcs"
	}
	if strings.Contains(lower, "psg") || strings.Contains(lower, "pasang") {
		return "pasang"
	}
	if strings.Contains(lower, "lsn") || strings.Contains(lower, "lusin") {
		return "lsn"
	}
	if strings.Contains(lower, "dus") || strings.Contains(lower, "box") {
		return "dus"
	}
	if strings.Contains(lower, "rol") {
		return "rol"
	}
	if strings.Contains(lower, "set") {
		return "set"
	}
	return ""
}

func parsePriceWithBasis(raw string) (*float64, string) {
	clean := strings.TrimSpace(raw)
	basis := ""

	// Check for suffixes like /kg, /lb, /pcs, /mm, /psg, /m³
	if idx := strings.Index(clean, "/"); idx != -1 {
		basis = strings.TrimSpace(clean[idx:])
		clean = strings.TrimSpace(clean[:idx])
	}

	// Remove thousands dots
	clean = strings.ReplaceAll(clean, ".", "")
	// Replace comma decimal with dot
	clean = strings.ReplaceAll(clean, ",", ".")

	// Extract digits only
	re := regexp.MustCompile(`[0-9]+(\.[0-9]+)?`)
	match := re.FindString(clean)
	if match == "" {
		return nil, basis
	}

	val, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return nil, basis
	}
	return &val, basis
}

func parseStokBarangDepan(f *excelize.File, sheet string) []OpeningStockItem {
	var items []OpeningStockItem
	rows, err := f.GetRows(sheet)
	if err != nil {
		return items
	}

	for idx, row := range rows {
		rowNum := idx + 1
		if len(row) < 3 {
			continue
		}
		name := strings.TrimSpace(row[0])
		if name == "" || strings.EqualFold(name, "Nama Barang") {
			continue
		}
		brand := strings.TrimSpace(row[1])
		rawStock := strings.TrimSpace(row[2])

		item := OpeningStockItem{
			RowIndex: rowNum,
			Name:     name,
			Brand:    brand,
			RawStock: rawStock,
		}

		if rawStock == "" {
			item.Status = "UNCOUNTED"
		} else if strings.Contains(rawStock, "+") {
			item.Status = "AMBIGUOUS"
			item.IsAmbiguous = true
		} else {
			// Extract quantity and unit
			re := regexp.MustCompile(`([0-9]+)\s*([a-zA-Z]+)?`)
			matches := re.FindStringSubmatch(rawStock)
			if len(matches) > 1 {
				qty, err := strconv.ParseFloat(matches[1], 64)
				if err == nil {
					item.ParsedQty = &qty
					item.Status = "RELIABLE"
					if len(matches) > 2 && matches[2] != "" {
						item.Unit = strings.ToLower(matches[2])
					} else {
						item.Unit = "pcs"
					}
				} else {
					item.Status = "AMBIGUOUS"
				}
			} else {
				item.Status = "AMBIGUOUS"
			}
		}

		items = append(items, item)
	}

	return items
}

func parsePasirSplit(f *excelize.File, sheet string) []BulkMaterialItem {
	var items []BulkMaterialItem
	rows, err := f.GetRows(sheet)
	if err != nil {
		return items
	}

	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		name := strings.TrimSpace(row[0])
		if name == "" {
			continue
		}
		rawPrice := ""
		if len(row) > 1 {
			rawPrice = strings.TrimSpace(row[1])
		}

		val, _ := parsePriceWithBasis(rawPrice)
		items = append(items, BulkMaterialItem{
			Name:     name,
			RawPrice: rawPrice,
			Price:    val,
			Note:     "Bulk building material (Pasir/Hebel/Split)",
		})
	}
	return items
}

func strVal(s *string, fallback string) string {
	if s != nil {
		return *s
	}
	return fallback
}

func floatVal(f *float64) float64 {
	if f != nil {
		return *f
	}
	return 0
}

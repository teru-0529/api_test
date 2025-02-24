package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	"github.com/teru-0529/api-test/api"
	"github.com/teru-0529/api-test/fixture"
	"github.com/teru-0529/api-test/verification"
)

const FIXTURE_DIR = "./testdata/fixture/"
const GOLDEN_DIR = "./testdata/golden/"
const SPEC_DIR = "./testdata/testspec/"
const TEST_SETTING_PATH = "./testdata/testSetting.yaml"

// TEST: API実行テスト
func TestApi(t *testing.T) {
	// PROCESS: configの呼び出し
	leadEnv()
	apiAccesser := api.New()
	setting, err := fixture.NewSettings(TEST_SETTING_PATH)
	if err != nil {
		t.Fatal(err)
	}

	files, _ := os.ReadDir(FIXTURE_DIR)
	for _, file := range files {
		file := file
		fileKey := file.Name()[:strings.Index(file.Name(), ".")]
		update := slices.Contains(setting.UpdateGorden, file.Name())
		verify := true

		// PROCESS: fixtureの生成
		fix, err := fixture.New(path.Join(FIXTURE_DIR, file.Name()))
		if err != nil {
			t.Errorf("fixture parse error[%s]: %v", file.Name(), err)
			log.Printf("fixture parse error[%s]: %v, so this test skipped.", file.Name(), err)
			continue
		}

		log.Println(fix.Name)
		if slices.Contains(setting.WipList, file.Name()) {
			log.Println(" - (*) work in progress, so this test skipped.")
			continue
		}

		t.Run(fix.Name, func(t *testing.T) {

			// PROCESS: 部分テストのケースでwhiteListに存在しなければskip
			if setting.PartialTest && !slices.Contains(setting.WhiteList, file.Name()) {
				log.Println(" - (*) whitelist test execute, so this test skipped.")
				t.Skip("skipped the test.")
			}

			if update {
				log.Println(" - (*) golden file update.")
			}

			// PROCESS: Dbのリセット(対象テーブルのtruncate/sequenceの初期化)
			if err = apiAccesser.Reset(fix.Reset); err != nil {
				log.Println(" - reset(before) NG")
				t.Fatalf("reset failured: (%v).", err)
			}
			log.Println(" - reset(before) OK")

			// PROCESS: テストデータのInsert
			for _, item := range fix.Setup {
				if err = apiAccesser.BulkInsert(item.Schema, item.Table, item.Body); err != nil {
					log.Println(" - setupTable NG")
					log.Printf("   - %v", err)
					t.Fatalf("setup failured: (%v).", err)
				}
			}
			log.Println(" - setupTable OK")

			// PROCESS: API実行
			res, status, err := apiAccesser.Execute(fix.Execute)
			if err != nil {
				log.Println(" - execute NG")
				log.Printf("   - %v", err)
				t.Fatalf("execute failured: (%v).", err)
			}
			log.Println(" - execute OK")

			// PROCESS: 検証1 :HttpStaus
			if status != fix.Verification.HttpStatus {
				t.Errorf("HttpStatus are not correct. expected: %v, got: %v:", fix.Verification.HttpStatus, status)
				verify = false
			}

			// PROCESS: 検証2 :レスポンスBody
			if fix.Verification.Result.IsCheck {
				goldenFile := path.Join(GOLDEN_DIR, fmt.Sprintf("%s.golden", fileKey))
				verify = verify && verification.JsonVerify(t, res, goldenFile, update, fix.Verification.Result.Excludes, "api response")
			}

			// PROCESS: 検証3 :Database
			for _, table := range fix.Verification.Tables {
				res, err := apiAccesser.GetAll(table.Schema, table.Table)
				if err != nil {
					log.Println(" - verification NG")
					log.Printf("   - %v", err)
					t.Fatalf("verification failured: (%v).", err)
				}
				key := fmt.Sprintf("table:: %s.%s", table.Schema, table.Table)
				goldenFile := path.Join(GOLDEN_DIR, fmt.Sprintf("%s-%s-%s.golden", fileKey, table.Schema, table.Table))
				verify = verify && verification.JsonVerify(t, res, goldenFile, update, table.Excludes, key)
			}
			if verify {
				log.Println(" - verification OK")
			} else {
				log.Println(" - verification OK / (*) failure the test.")
			}
			fix.WriteSpecification(path.Join(SPEC_DIR, fmt.Sprintf("%s.md", fileKey)))

			// PROCESS: 後処理(対象テーブルのtruncate/sequenceの初期化)
			if err = apiAccesser.Reset(fix.Reset); err != nil {
				log.Println(" - reset(after) NG")
				t.Fatalf("reset failured: (%v).", err)
			}
			log.Println(" - reset(after) OK")
		})

	}
}

// FUNCTION: 環境変数への設定(.envファイルがある場合のみ)
func leadEnv() {
	// envファイルのロード
	_, err := os.Stat(".env")
	if !os.IsNotExist(err) {
		godotenv.Load()
		log.Print("loaded environment variables from .env file.")
	}
}

package main

import (
	"fmt"
	"os"

	"intent-lang/internal/similarity"
)

func compare(args []string) error {
	f := flags(args)
	refFile := f["ref"]
	candFile := f["cand"]
	if refFile == "" || candFile == "" {
		return fmt.Errorf("compare requires --ref <file> --cand <file>")
	}
	ref, err := os.ReadFile(refFile)
	if err != nil {
		return err
	}
	cand, err := os.ReadFile(candFile)
	if err != nil {
		return err
	}

	r := similarity.Compare(string(ref), string(cand))
	fmt.Println(r)

	threshold := 0.95
	if v := f["threshold"]; v != "" {
		if _, err := fmt.Sscanf(v, "%f", &threshold); err != nil {
			return fmt.Errorf("bad threshold: %v", err)
		}
	}

	fidelity := f["fidelity"]
	if fidelity == "" {
		fidelity = "structure"
	}
	if fidelity == "behavior" {
		fmt.Println("提示: FIDELITY=behavior 时文本相似度不代表行为一致；应改用 ACCEPT 行为验收（参见 testdata 验证：文本 0.127 但行为用例全过）。")
		return nil
	}

	if r.Pass(threshold) {
		fmt.Printf("PASS: 相似度 %.3f >= %.2f\n", r.Score(), threshold)
		return nil
	}
	fmt.Printf("FAIL: 相似度 %.3f < %.2f\n", r.Score(), threshold)
	os.Exit(1)
	return nil
}

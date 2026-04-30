package mysql

import (
	"testing"
)

func TestSplitStatements_simple(t *testing.T) {
	input := "CREATE TABLE a (id INT);\nCREATE TABLE b (id INT);\n"
	stmts := splitStatements(input)
	if len(stmts) != 2 {
		t.Fatalf("want 2 statements, got %d: %v", len(stmts), stmts)
	}
	if stmts[0] != "CREATE TABLE a (id INT)" {
		t.Errorf("stmt[0] = %q", stmts[0])
	}
	if stmts[1] != "CREATE TABLE b (id INT)" {
		t.Errorf("stmt[1] = %q", stmts[1])
	}
}

func TestSplitStatements_delimiterSwitch(t *testing.T) {
	input := "CREATE TABLE x (id INT);\n" +
		"DELIMITER $$\n" +
		"CREATE TRIGGER t BEFORE INSERT ON x\n" +
		"FOR EACH ROW\n" +
		"BEGIN\n" +
		"    SET NEW.id = 1;\n" +
		"END$$\n" +
		"DELIMITER ;\n" +
		"CREATE TABLE y (id INT);\n"

	stmts := splitStatements(input)
	if len(stmts) != 3 {
		t.Fatalf("want 3 statements, got %d:\n%v", len(stmts), stmts)
	}
	if stmts[0] != "CREATE TABLE x (id INT)" {
		t.Errorf("stmt[0] = %q", stmts[0])
	}
	if stmts[2] != "CREATE TABLE y (id INT)" {
		t.Errorf("stmt[2] = %q", stmts[2])
	}
}

func TestSplitStatements_commentsIgnored(t *testing.T) {
	input := "-- This is a comment\nCREATE TABLE z (id INT);\n"
	stmts := splitStatements(input)
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_empty(t *testing.T) {
	stmts := splitStatements("")
	if len(stmts) != 0 {
		t.Fatalf("want 0 statements for empty input, got %d", len(stmts))
	}
}

func TestSplitStatements_whitespaceOnly(t *testing.T) {
	stmts := splitStatements("   \n\n  \n")
	if len(stmts) != 0 {
		t.Fatalf("want 0 statements for whitespace-only input, got %d", len(stmts))
	}
}

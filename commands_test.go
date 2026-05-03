package main

import "testing"

func TestParseCommand_Empty(t *testing.T) {
	cmd, args := parseCommand("")
	if cmd != "" {
		t.Errorf("expected empty cmd, got %q", cmd)
	}
	if args != nil {
		t.Errorf("expected nil args, got %v", args)
	}
}

func TestParseCommand_NoArgs(t *testing.T) {
	cmd, args := parseCommand("convert")
	if cmd != "convert" {
		t.Errorf("expected convert, got %q", cmd)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %v", args)
	}
}

func TestParseCommand_WithArgs(t *testing.T) {
	cmd, args := parseCommand("cd /home/user")
	if cmd != "cd" {
		t.Errorf("expected cd, got %q", cmd)
	}
	if len(args) != 1 || args[0] != "/home/user" {
		t.Errorf(`expected ["/home/user"], got %v`, args)
	}
}

func TestParseCommand_Whitespace(t *testing.T) {
	cmd, args := parseCommand("  tag  ")
	if cmd != "tag" {
		t.Errorf("expected tag, got %q", cmd)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %v", args)
	}
}

func TestCommandsStartingWith_Edit(t *testing.T) {
	matches := commandsStartingWith("ed")
	if len(matches) != 1 || matches[0] != "edit" {
		t.Errorf("expected [edit], got %v", matches)
	}
}

func TestCommandsStartingWith_EditVsEmpty(t *testing.T) {
	matches := commandsStartingWith("e")
	if len(matches) != 1 || matches[0] != "edit" {
		t.Errorf("expected [edit], got %v", matches)
	}
}

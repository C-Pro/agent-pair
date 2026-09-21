package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// NewTurnID returns a short random identifier that tags one exchange. Every
// turn carries its own id so a marker left in scrollback, or a marker inside a
// quoted earlier reply, cannot be mistaken for the completion of this turn.
func NewTurnID() string {
	var buf [2]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Randomness is a de-duplication aid, not a security control; a fixed
		// id only degrades detection to the pre-nonce behaviour.
		return "0000"
	}
	return hex.EncodeToString(buf[:])
}

// FormatTurn wraps message with radio turn markers tagged by turnID.
func FormatTurn(sender, recipient, message, turnID string) string {
	msg := strings.TrimSpace(message)

	openingPrefix := fmt.Sprintf("[ CQ %s -> %s #%s ]", sender, recipient, turnID)
	closingSuffix := fmt.Sprintf("[ %s over #%s ]", sender, turnID)

	var sb strings.Builder
	sb.WriteString(openingPrefix)
	sb.WriteString("\n\n")
	sb.WriteString(msg)
	sb.WriteString("\n\n")
	sb.WriteString(closingSuffix)

	return sb.String()
}

// FormatBootstrapPrompt creates the initial instruction given to the follower agent.
func FormatBootstrapPrompt(leaderCallsign, followerCallsign, workingDir, turnID string, readOnly bool) string {
	roConstraint := ""
	if readOnly {
		roConstraint = `- You have READ-ONLY permissions. DO NOT create, modify, or delete any files, and DO NOT run destructive shell commands, execute or compile code. You can read, inspect, and analyze code freely.
- You may ASK the lead agent to run benign verification steps on your behalf: linting, type checking, compiling, or running the test suite. State the exact command and why you want it. The lead decides whether to run it, and will refuse anything that writes outside the workspace, touches production, moves data off this machine, or has side effects beyond the build and test caches.
`
	}

	return fmt.Sprintf(`[ PROTOCOL INITIALIZATION ]
You are participating in an agentic pair programming session.
- Your callsign: %s
- Your partner (Lead Agent): %s
- Working Directory: %s
%s
COMMUNICATION PROTOCOL RULES:
1. Every message must use this envelope, replacing the placeholders with the callsigns above:
   [ CQ {your-callsign} -> {partner-callsign} #{turn-id} ]
   {response body}
   [ {your-callsign} over #{turn-id} ]
2. {turn-id} is the id printed in the header of the message you are replying to. Copy it exactly. This turn's id is %s.
3. When ending the entire session, replace "over" with "out".
4. Write the closing marker on its own line, and only once you have finished the whole reply.
5. Be concise, direct, and constructive. Provide concrete code suggestions, architectural critique, and alternatives.

Please acknowledge receipt now using that envelope and this response body:
Ready for pair programming as %s. Standing by.`,
		followerCallsign,
		leaderCallsign,
		workingDir,
		roConstraint,
		turnID,
		followerCallsign,
	)
}

// turnEndPattern builds the regex matching a sender's end-of-turn marker.
// Brackets are required, or the marker must sit alone on its line: a bare
// "<callsign> over" inside a sentence ("I'd pick claude over codex") is prose,
// not a marker. When turnID is set the marker must carry that id.
func turnEndPattern(sender, turnID string) *regexp.Regexp {
	id := `(?:\s*#\s*[0-9a-zA-Z]+)?`
	if turnID != "" {
		id = `\s*#\s*` + regexp.QuoteMeta(turnID)
	}
	s := regexp.QuoteMeta(sender)
	return regexp.MustCompile(fmt.Sprintf(
		`(?im)(?:\[\s*%s\s+(?:over|out)%s\s*\]|^\s*%s\s+(?:over|out)%s\s*$)`,
		s, id, s, id,
	))
}

// scanWindow narrows the captured screen to the text that follows the echo of
// the leader's own prompt for this turn. Everything the leader sent -- including
// any earlier reply it quoted back -- sits before that point, so markers inside
// the prompt cannot complete the turn. Returns false when the echo is not on
// screen (the prompt scrolled off, or nothing was sent yet).
func scanWindow(cleaned, leaderCallsign, turnID string) (string, bool) {
	if turnID == "" {
		return cleaned, false
	}
	marker := turnEndPattern(leaderCallsign, turnID)
	matches := marker.FindAllStringIndex(cleaned, -1)
	if len(matches) == 0 {
		return cleaned, false
	}
	return cleaned[matches[len(matches)-1][1]:], true
}

// IsTurnComplete reports whether the follower has finished the turn identified
// by turnID. An id-tagged marker is unambiguous and counts wherever it appears.
// Inside the window that follows the leader's echoed prompt an untagged marker
// is accepted too, for followers that drop the id. With neither, the turn is
// not complete.
func IsTurnComplete(screen, followerCallsign, leaderCallsign, turnID string) bool {
	cleaned := StripANSI(screen)
	window, windowed := scanWindow(cleaned, leaderCallsign, turnID)

	if turnID != "" && turnEndPattern(followerCallsign, turnID).MatchString(window) {
		return true
	}
	if !windowed {
		return false
	}
	return turnEndPattern(followerCallsign, "").MatchString(window)
}

// ExtractLatestTurn finds the latest message sent by sender to recipient.
func ExtractLatestTurn(screen, sender, recipient, turnID string) (string, bool) {
	cleaned := CleanTUIArtifacts(screen)
	if window, windowed := scanWindow(cleaned, recipient, turnID); windowed {
		cleaned = window
	}

	id := `(?:\s*#\s*[0-9a-zA-Z]+)?`
	qs := regexp.QuoteMeta(sender)
	qr := regexp.QuoteMeta(recipient)

	// 1. Strict: [ CQ <sender> -> <recipient> #id ] ... [ <sender> over #id ]
	strictRe := regexp.MustCompile(fmt.Sprintf(`(?s)\[\s*CQ\s+%s\s*->\s*%s%s\s*\]\s*(.*?)\s*\[\s*%s\s+(?:over|out)%s\s*\]`, qs, qr, id, qs, id))
	if matches := strictRe.FindAllStringSubmatch(cleaned, -1); len(matches) > 0 {
		return strings.TrimSpace(matches[len(matches)-1][1]), true
	}

	// 2. Relaxed, for TUIs that strip the brackets out of the envelope.
	relaxedRe := regexp.MustCompile(fmt.Sprintf(`(?si)(?:\[\s*)?CQ\s+%s\s*->\s*%s%s(?:\s*\])?\s*(.*?)\s*(?:\[\s*)?%s\s+(?:over|out)%s(?:\s*\])?`, qs, qr, id, qs, id))
	if matches := relaxedRe.FindAllStringSubmatch(cleaned, -1); len(matches) > 0 {
		return strings.TrimSpace(matches[len(matches)-1][1]), true
	}

	// 3. Fallback: the text preceding the sender's closing marker.
	overRe := regexp.MustCompile(fmt.Sprintf(`(?si)(.*?)\s*(?:\[\s*)?%s\s+(?:over|out)%s(?:\s*\])?`, qs, id))
	if matches := overRe.FindAllStringSubmatch(cleaned, -1); len(matches) > 0 {
		last := matches[len(matches)-1][1]
		if idx := strings.LastIndex(last, fmt.Sprintf("-> %s", sender)); idx != -1 {
			last = last[idx:]
			if newlineIdx := strings.Index(last, "\n"); newlineIdx != -1 {
				last = last[newlineIdx:]
			}
		}
		if body := strings.TrimSpace(last); len(body) > 0 {
			return body, true
		}
	}

	return "", false
}

// HasTurnFinished checks if the screen indicates the sender finished their turn.
// turnID may be empty, which matches a marker carrying any id or none.
func HasTurnFinished(screen, sender, turnID string) bool {
	return turnEndPattern(sender, turnID).MatchString(StripANSI(screen))
}

// CountTurnEndMarkers returns the number of completed response markers from a
// sender in captured terminal output. Callers use a pre-send count as a
// watermark so markers left in scrollback cannot complete a later turn.
func CountTurnEndMarkers(screen, sender, turnID string) int {
	return len(turnEndPattern(sender, turnID).FindAllStringIndex(StripANSI(screen), -1))
}

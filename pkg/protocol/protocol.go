package protocol

import (
	"fmt"
	"regexp"
	"strings"
)

// FormatTurn wraps message with radio turn markers.
func FormatTurn(sender, recipient, message string) string {
	msg := strings.TrimSpace(message)

	openingPrefix := fmt.Sprintf("[ CQ %s -> %s ]", sender, recipient)
	closingSuffix := fmt.Sprintf("[ %s over ]", sender)

	var sb strings.Builder
	if !strings.HasPrefix(msg, "[ CQ ") {
		sb.WriteString(openingPrefix)
		sb.WriteString("\n\n")
	}

	sb.WriteString(msg)

	if !strings.HasSuffix(msg, " over ]") && !strings.HasSuffix(msg, " out ]") {
		sb.WriteString("\n\n")
		sb.WriteString(closingSuffix)
	}

	return sb.String()
}

// FormatBootstrapPrompt creates the initial instruction given to the follower agent.
func FormatBootstrapPrompt(leaderCallsign, followerCallsign, workingDir string, readOnly bool) string {
	roConstraint := ""
	if readOnly {
		roConstraint = "- You have READ-ONLY permissions. DO NOT create, modify, or delete any files, and DO NOT run destructive shell commands, execute or compile code. You can read, inspect, and analyze code freely.\n"
	}

	return fmt.Sprintf(`[ PROTOCOL INITIALIZATION ]
You are participating in an agentic pair programming session.
- Your callsign: %s
- Your partner (Lead Agent): %s
- Working Directory: %s
%s
COMMUNICATION PROTOCOL RULES:
1. Every message must use this envelope, replacing the placeholders with the callsigns above:
   [ CQ {your-callsign} -> {partner-callsign} ]
   {response body}
   [ {your-callsign} over ]
2. When ending the entire session, replace "over" with "out".
3. Be concise, direct, and constructive. Provide concrete code suggestions, architectural critique, and alternatives.

Please acknowledge receipt now using that envelope and this response body:
Ready for pair programming as %s. Standing by.`,
		followerCallsign,
		leaderCallsign,
		workingDir,
		roConstraint,
		followerCallsign,
	)
}

// ExtractLatestTurn finds the latest message sent by sender to recipient.
func ExtractLatestTurn(screen string, sender, recipient string) (string, bool) {
	cleaned := CleanTUIArtifacts(screen)

	// 1. Strict regex: [ CQ <sender> -> <recipient> ] ... [ <sender> over ]
	strictPattern := fmt.Sprintf(`(?s)\[\s*CQ\s+%s\s*->\s*%s\s*\]\s*(.*?)\s*\[\s*%s\s*(?:over|out)\s*\]`,
		regexp.QuoteMeta(sender),
		regexp.QuoteMeta(recipient),
		regexp.QuoteMeta(sender),
	)
	strictRe := regexp.MustCompile(strictPattern)
	if matches := strictRe.FindAllStringSubmatch(cleaned, -1); len(matches) > 0 {
		return strings.TrimSpace(matches[len(matches)-1][1]), true
	}

	// 2. Relaxed regex (where markdown may strip brackets)
	relaxedPattern := fmt.Sprintf(`(?si)(?:\[\s*)?CQ\s+%s\s*->\s*%s(?:\s*\])?\s*(.*?)\s*(?:\[\s*)?%s\s*(?:over|out)(?:\s*\])?`,
		regexp.QuoteMeta(sender),
		regexp.QuoteMeta(recipient),
		regexp.QuoteMeta(sender),
	)
	relaxedRe := regexp.MustCompile(relaxedPattern)
	if matches := relaxedRe.FindAllStringSubmatch(cleaned, -1); len(matches) > 0 {
		return strings.TrimSpace(matches[len(matches)-1][1]), true
	}

	// 3. Fallback: find text before '<sender> over' or '<sender> out'
	overPattern := fmt.Sprintf(`(?si)(.*?)\s*(?:\[\s*)?%s\s*(?:over|out)(?:\s*\])?`, regexp.QuoteMeta(sender))
	overRe := regexp.MustCompile(overPattern)
	if matches := overRe.FindAllStringSubmatch(cleaned, -1); len(matches) > 0 {
		last := matches[len(matches)-1][1]
		// Trim any leading prompt delimiter
		if idx := strings.LastIndex(last, fmt.Sprintf("-> %s", sender)); idx != -1 {
			last = last[idx:]
			if newlineIdx := strings.Index(last, "\n"); newlineIdx != -1 {
				last = last[newlineIdx:]
			}
		}
		body := strings.TrimSpace(last)
		if len(body) > 0 {
			return body, true
		}
	}

	return "", false
}

// HasTurnFinished checks if the screen indicates the sender finished their turn.
func HasTurnFinished(screen string, sender string) bool {
	cleaned := StripANSI(screen)
	overPattern := fmt.Sprintf(`(?i)(?:\[\s*%s\s*(?:over|out)\s*\]|\b%s\s+(?:over|out)\b)`,
		regexp.QuoteMeta(sender),
		regexp.QuoteMeta(sender),
	)
	re := regexp.MustCompile(overPattern)
	return re.MatchString(cleaned)
}

// CountTurnEndMarkers returns the number of completed response markers from a
// sender in captured terminal output. Callers use a pre-send count as a
// watermark so markers left in scrollback cannot complete a later turn.
func CountTurnEndMarkers(screen string, sender string) int {
	cleaned := StripANSI(screen)
	pattern := fmt.Sprintf(`(?i)(?:\[\s*%s\s*(?:over|out)\s*\]|\b%s\s+(?:over|out)\b)`,
		regexp.QuoteMeta(sender),
		regexp.QuoteMeta(sender),
	)
	return len(regexp.MustCompile(pattern).FindAllStringIndex(cleaned, -1))
}

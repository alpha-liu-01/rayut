package route

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/alpha-liu-01/rayut/daemon/internal/rules"
)

// Recover deletes only the mihomo policy rules observed on this phone and
// clears routing table 2022. It does not flush the main table or the firewall.
func Recover() error {
	for _, family := range []string{"4", "6"} {
		if err := deleteOwned(family); err != nil {
			return err
		}
		if err := reportUnexpected(family); err != nil {
			return err
		}
	}
	if err := flushTable("4"); err != nil {
		return err
	}
	return flushTable("6")
}

func deleteOwned(family string) error {
	for i := 0; i < 40; i++ {
		out, err := ruleShow(family)
		if err != nil {
			return err
		}
		priority := ""
		for _, line := range strings.Split(out, "\n") {
			prio, kind := rules.Classify(line)
			if kind == "owned" {
				priority = prio
				break
			}
		}
		if priority == "" {
			return nil
		}
		if err := ruleDel(family, priority); err != nil {
			return err
		}
	}
	return fmt.Errorf("too many owned rules for ipv%s", family)
}

func reportUnexpected(family string) error {
	out, err := ruleShow(family)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(out, "\n") {
		priority, kind := rules.Classify(line)
		if kind == "unexpected" {
			return fmt.Errorf("unexpected ipv%s rule at priority %s", family, priority)
		}
	}
	return nil
}

func ruleShow(family string) (string, error) {
	args := []string{"rule", "show"}
	if family == "6" {
		args = []string{"-6", "rule", "show"}
	}
	out, err := exec.Command("ip", args...).Output()
	if err != nil {
		return "", fmt.Errorf("ip rule show: %w", err)
	}
	return string(out), nil
}

func ruleDel(family, priority string) error {
	args := []string{"rule", "del", "pref", priority}
	if family == "6" {
		args = []string{"-6", "rule", "del", "pref", priority}
	}
	if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("ip rule del %s: %w (%s)", priority, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func flushTable(family string) error {
	args := []string{"route", "flush", "table", "2022"}
	if family == "6" {
		args = []string{"-6", "route", "flush", "table", "2022"}
	}
	if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
		text := strings.TrimSpace(string(out))
		if strings.Contains(text, "FIB table does not exist") || text == "" {
			return nil
		}
		return fmt.Errorf("flush table 2022: %w (%s)", err, text)
	}
	return nil
}

// TunPresent reports whether the mihomo interface from the day-2 config exists.
func TunPresent() (bool, error) {
	out, err := exec.Command("ip", "-o", "link", "show").Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), " Meta:"), nil
}

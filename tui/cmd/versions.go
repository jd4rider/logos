package cmd

import (
"fmt"

"github.com/spf13/cobra"
)

var versionsCmd = &cobra.Command{
Use:   "versions",
Short: "List available Bible translations",
RunE: func(cmd *cobra.Command, args []string) error {
lang, _ := cmd.Flags().GetString("language")
bibles, err := client.GetBibles(lang)
if err != nil {
return fmt.Errorf("fetching bibles: %w", err)
}
fmt.Printf("%-36s  %-10s  %s\n", "ID", "Abbrev", "Name")
fmt.Println(fmt.Sprintf("%s  %s  %s", repeat("-", 36), repeat("-", 10), repeat("-", 40)))
for _, b := range bibles {
fmt.Printf("%-36s  %-10s  %s\n", b.ID, b.Abbreviation, b.Name)
}
return nil
},
}

func repeat(s string, n int) string {
out := ""
for i := 0; i < n; i++ {
out += s
}
return out
}

func init() {
versionsCmd.Flags().StringP("language", "l", "eng", "Language filter (e.g. eng, spa, fra)")
rootCmd.AddCommand(versionsCmd)
}

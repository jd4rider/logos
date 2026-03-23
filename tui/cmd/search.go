package cmd

import (
"fmt"

"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
Use:   "search [query]",
Short: "Search the Bible for verses",
Args:  cobra.ExactArgs(1),
RunE: func(cmd *cobra.Command, args []string) error {
query := args[0]
bibleID, _ := cmd.Flags().GetString("bible")
if bibleID == "" {
bibleID = "de4e12af7f28f599-02"
}
limit, _ := cmd.Flags().GetInt("limit")

data, err := client.Search(bibleID, query, limit)
if err != nil {
return fmt.Errorf("search failed: %w", err)
}

fmt.Printf("Found %d results for %q:\n\n", data.Total, query)
for i, v := range data.Verses {
fmt.Printf("%d. %s\n   %s\n\n", i+1, v.Reference, v.Text)
}
return nil
},
}

func init() {
searchCmd.Flags().StringP("bible", "b", "", "Bible ID (default: KJV)")
searchCmd.Flags().IntP("limit", "l", 20, "Maximum results")
rootCmd.AddCommand(searchCmd)
}

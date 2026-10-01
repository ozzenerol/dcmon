package main

import "dcmon/cmd"

func main() {
	cfg := cmd.CmdConfig{
		Symbol:      "$ ",
		SaveHistory: true,
		HistoryPath: ".history",
		CtrlCAborts: true,
	}
	cmd.Execute(cfg)

}

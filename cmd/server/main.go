// Copyright 2022 NLP Odyssey Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/joho/godotenv"
	"github.com/nlpodyssey/cybertron/pkg/server"
	"github.com/nlpodyssey/cybertron/pkg/tasks"
	"github.com/nlpodyssey/cybertron/pkg/tasks/questionanswering"
	"github.com/nlpodyssey/cybertron/pkg/tasks/text2text"
	"github.com/nlpodyssey/cybertron/pkg/tasks/textclassification"
	"github.com/nlpodyssey/cybertron/pkg/tasks/zeroshotclassifier"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const defaultModelsDir = "models"
const addrRandomPort = ":0"

// main is the entry point of the application.
func main() {
	if err := run(); err != nil {
		log.Error().Err(err).Send()
		os.Exit(1)
	}
}

// run set the configuration and starts the server.
func run() error {
	initLogger()
	loadDotenv()

	conf := &config{
		loaderConfig: &tasks.Config{ModelsDir: defaultModelsDir},
		serverConfig: &server.Config{Address: addrRandomPort},
	}

	// load env vars values *before* parsing command line flags:
	// this gives to the flag a priority over values from the environment.
	if err := conf.loadEnv(); err != nil {
		return err
	}

	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	conf.bindFlagSet(fs)

	err := fs.Parse(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}

	m, err := loadModelForTask(conf)
	if err != nil {
		return err
	}
	defer tasks.Finalize(m)

	taskServer, err := makeTaskServer(m)
	if err != nil {
		return err
	}

	s := server.New(conf.serverConfig, taskServer)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	return s.Start(ctx)
}

func loadModelForTask(conf *config) (m any, err error) {
	switch conf.task {
	case ZeroShotClassificationTask:
		return tasks.Load[zeroshotclassifier.Interface](conf.loaderConfig)
	case Text2TextTask:
		return tasks.Load[text2text.Interface](conf.loaderConfig)
	case QuestionAnsweringTask:
		return tasks.Load[questionanswering.Interface](conf.loaderConfig)
	case TextClassificationTask:
		return tasks.Load[textclassification.Interface](conf.loaderConfig)
	default:
		return nil, fmt.Errorf("failed to load model/task type %s", conf.task)
	}
}

// makeTaskServer instantiates a new task-server based on the model.
func makeTaskServer(model any) (server.TaskServer, error) {
	switch m := model.(type) {
	case text2text.Interface:
		return server.NewServerForTextGeneration(m), nil
	case zeroshotclassifier.Interface:
		return server.NewServerForZeroShotClassification(m), nil
	case questionanswering.Interface:
		return server.NewServerForQuestionAnswering(m), nil
	case textclassification.Interface:
		return server.NewServerForTextClassification(m), nil
	default:
		return nil, fmt.Errorf("failed to resolve register funcs for model/task type %T", m)
	}
}

// initLogger initializes the logger.
func initLogger() {
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

// loadDotenv loads the .env file if it exists.
func loadDotenv() {
	_, err := os.Stat(".env")
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		log.Warn().Err(err).Msg("failed to read .env file")
		return
	}
	err = godotenv.Load()
	if err != nil {
		log.Warn().Err(err).Msg("failed to read .env file")
	}
}

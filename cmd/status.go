// Copyright 2020 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/status"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/spf13/cobra"
)

//Status returns the status of the synchronization process
func Status() *cobra.Command {
	var devPath string
	var namespace string
	var showInfo bool
	var watch bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: fmt.Sprintf("Status of the synchronization process"),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("starting status command")

			if k8Client.InCluster() {
				return errors.ErrNotInCluster
			}

			dev, err := utils.LoadDev(devPath)
			if err != nil {
				return err
			}
			if err := dev.UpdateNamespace(namespace); err != nil {
				return err
			}

			_, _, namespace, err = k8Client.GetLocal()
			if err != nil {
				return err
			}

			if dev.Namespace == "" {
				dev.Namespace = namespace
			}

			sy, err := syncthing.Load(dev)
			if err != nil {
				log.Debugf("error accessing to syncthing info file: %s", err)
				return errors.ErrNotInDevMode
			}
			if showInfo {
				log.Information("Local syncthing url: http://%s", sy.GUIAddress)
				log.Information("Remote syncthing url: http://%s", sy.RemoteGUIAddress)
				log.Information("Syncthing username: okteto")
				log.Information("Syncthing password: %s", sy.GUIPassword)
			}

			ctx := context.Background()
			if watch {
				err = runWithWatch(ctx, dev, sy)
			} else {
				err = runWithoutWatch(ctx, dev, sy)
			}

			analytics.TrackStatus(err == nil, showInfo)
			return err
		},
	}
	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executing")
	cmd.Flags().BoolVarP(&showInfo, "info", "i", false, "show syncthing links for troubleshooting the synchronization service")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "watch for changes")
	return cmd
}

func runWithWatch(ctx context.Context, dev *model.Dev, sy *syncthing.Syncthing) error {
	postfix := "Synchronizing your files..."
	spinner := utils.NewSpinner(postfix)
	pbScaling := 0.30
	spinner.Start()
	defer spinner.Stop()
	for {
		message := ""
		progress, err := status.Run(ctx, dev, sy)
		if err != nil {
			return err
		}
		if progress == 100 {
			message = "Files synchronized"
		} else {
			message = renderProgressBar(postfix, progress, pbScaling)
		}
		spinner.Update(message)
		time.Sleep(2 * time.Second)
	}
}

func runWithoutWatch(ctx context.Context, dev *model.Dev, sy *syncthing.Syncthing) error {
	progress, err := status.Run(ctx, dev, sy)
	if err != nil {
		return err
	}
	if progress == 100 {
		log.Success("Synchronization status: %.2f%%", progress)
	} else {
		log.Yellow("Synchronization status: %.2f%%", progress)
	}
	return nil
}

package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/authelia/authelia/v4/internal/configuration"
	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/configuration/validator"
	"github.com/authelia/authelia/v4/internal/models"
	"github.com/authelia/authelia/v4/internal/storage"
	"github.com/authelia/authelia/v4/internal/totp"
)

func storagePersistentPreRunE(cmd *cobra.Command, _ []string) (err error) {
	configs, err := cmd.Flags().GetStringSlice("config")
	if err != nil {
		return err
	}

	sources := make([]configuration.Source, 0, len(configs)+3)

	if cmd.Flags().Changed("config") {
		for _, configFile := range configs {
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				return fmt.Errorf("could not load the provided configuration file %s: %w", configFile, err)
			}

			sources = append(sources, configuration.NewYAMLFileSource(configFile))
		}
	} else {
		if _, err := os.Stat(configs[0]); err == nil {
			sources = append(sources, configuration.NewYAMLFileSource(configs[0]))
		}
	}

	mapping := map[string]string{
		"encryption-key": "storage.encryption_key",
		"sqlite.path":    "storage.local.path",

		"mysql.host":     "storage.mysql.host",
		"mysql.port":     "storage.mysql.port",
		"mysql.database": "storage.mysql.database",
		"mysql.username": "storage.mysql.username",
		"mysql.password": "storage.mysql.password",

		"postgres.host":                 "storage.postgres.host",
		"postgres.port":                 "storage.postgres.port",
		"postgres.database":             "storage.postgres.database",
		"postgres.schema":               "storage.postgres.schema",
		"postgres.username":             "storage.postgres.username",
		"postgres.password":             "storage.postgres.password",
		"postgres.ssl.mode":             "storage.postgres.ssl.mode",
		"postgres.ssl.root_certificate": "storage.postgres.ssl.root_certificate",
		"postgres.ssl.certificate":      "storage.postgres.ssl.certificate",
		"postgres.ssl.key":              "storage.postgres.ssl.key",

		"period":    "totp.period",
		"digits":    "totp.digits",
		"algorithm": "totp.algorithm",
		"issuer":    "totp.issuer",
	}

	sources = append(sources, configuration.NewEnvironmentSource(configuration.DefaultEnvPrefix, configuration.DefaultEnvDelimiter))
	sources = append(sources, configuration.NewSecretsSource(configuration.DefaultEnvPrefix, configuration.DefaultEnvDelimiter))
	sources = append(sources, configuration.NewCommandLineSourceWithMapping(cmd.Flags(), mapping, true, false))

	val := schema.NewStructValidator()

	config = &schema.Configuration{}

	_, err = configuration.LoadAdvanced(val, "", &config, sources...)
	if err != nil {
		return err
	}

	if val.HasErrors() {
		var finalErr error

		for i, err := range val.Errors() {
			if i == 0 {
				finalErr = err
				continue
			}

			finalErr = fmt.Errorf("%w, %v", finalErr, err)
		}

		return finalErr
	}

	validator.ValidateStorage(config.Storage, val)

	validator.ValidateTOTP(config, val)

	if val.HasErrors() {
		var finalErr error

		for i, err := range val.Errors() {
			if i == 0 {
				finalErr = err
				continue
			}

			finalErr = fmt.Errorf("%w, %v", finalErr, err)
		}

		return finalErr
	}

	return nil
}

func storageSchemaEncryptionCheckRunE(cmd *cobra.Command, args []string) (err error) {
	var (
		provider storage.Provider
		ctx      = context.Background()
	)

	provider = getStorageProvider()

	defer func() {
		_ = provider.Close()
	}()

	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return err
	}

	if err = provider.SchemaEncryptionCheckKey(ctx, verbose); err != nil {
		switch {
		case errors.Is(err, storage.ErrSchemaEncryptionVersionUnsupported):
			fmt.Printf("Could not check encryption key for validity. The schema version doesn't support encryption.\n")
		case errors.Is(err, storage.ErrSchemaEncryptionInvalidKey):
			fmt.Printf("Encryption key validation: failed.\n\nError: %v.\n", err)
		default:
			fmt.Printf("Could not check encryption key for validity.\n\nError: %v.\n", err)
		}
	} else {
		fmt.Println("Encryption key validation: success.")
	}

	return nil
}

func storageSchemaEncryptionChangeKeyRunE(cmd *cobra.Command, args []string) (err error) {
	var (
		provider storage.Provider
		ctx      = context.Background()
	)

	provider = getStorageProvider()

	defer func() {
		_ = provider.Close()
	}()

	if err = checkStorageSchemaUpToDate(ctx, provider); err != nil {
		return err
	}

	version, err := provider.SchemaVersion(ctx)
	if err != nil {
		return err
	}

	if version <= 0 {
		return errors.New("schema version must be at least version 1 to change the encryption key")
	}

	key, err := cmd.Flags().GetString("new-encryption-key")
	if err != nil {
		return err
	}

	if key == "" {
		return errors.New("you must set the --new-encryption-key flag")
	}

	if len(key) < 20 {
		return errors.New("the encryption key must be at least 20 characters")
	}

	if err = provider.SchemaEncryptionChangeKey(ctx, key); err != nil {
		return err
	}

	fmt.Println("Completed the encryption key change. Please adjust your configuration to use the new key.")

	return nil
}

func storageTOTPGenerateRunE(cmd *cobra.Command, args []string) (err error) {
	var (
		provider storage.Provider
		ctx      = context.Background()
		c        *models.TOTPConfiguration
		force    bool
	)

	provider = getStorageProvider()

	defer func() {
		_ = provider.Close()
	}()

	force, err = cmd.Flags().GetBool("force")

	_, err = provider.LoadTOTPConfiguration(ctx, args[0])
	if err == nil && !force {
		return fmt.Errorf("%s already has a TOTP configuration, use --force to overwrite", args[0])
	}

	if err != nil && !errors.Is(err, storage.ErrNoTOTPConfiguration) {
		return err
	}

	totpProvider := totp.NewTimeBasedProvider(config.TOTP)

	if c, err = totpProvider.Generate(args[0]); err != nil {
		return err
	}

	err = provider.SaveTOTPConfiguration(ctx, *c)
	if err != nil {
		return err
	}

	fmt.Printf("Generated TOTP configuration for user '%s': %s", args[0], c.URI())

	return nil
}

func storageTOTPDeleteRunE(cmd *cobra.Command, args []string) (err error) {
	var (
		provider storage.Provider
		ctx      = context.Background()
	)

	user := args[0]

	provider = getStorageProvider()

	defer func() {
		_ = provider.Close()
	}()

	_, err = provider.LoadTOTPConfiguration(ctx, user)
	if err != nil {
		return fmt.Errorf("can't delete configuration for user '%s': %+v", user, err)
	}

	err = provider.DeleteTOTPConfiguration(ctx, user)
	if err != nil {
		return fmt.Errorf("can't delete configuration for user '%s': %+v", user, err)
	}

	fmt.Printf("Deleted TOTP configuration for user '%s'.", user)

	return nil
}

func storageTOTPExportRunE(cmd *cobra.Command, args []string) (err error) {
	var (
		provider storage.Provider
		ctx      = context.Background()
	)

	provider = getStorageProvider()

	defer func() {
		_ = provider.Close()
	}()

	if err = checkStorageSchemaUpToDate(ctx, provider); err != nil {
		return err
	}

	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}

	switch format {
	case storageExportFormatCSV, storageExportFormatURI:
		break
	default:
		return errors.New("format must be csv or uri")
	}

	limit := 10

	var configurations []models.TOTPConfiguration

	for page := 0; true; page++ {
		configurations, err = provider.LoadTOTPConfigurations(ctx, limit, page)
		if err != nil {
			return err
		}

		if page == 0 && format == storageExportFormatCSV {
			fmt.Printf("issuer,username,algorithm,digits,period,secret\n")
		}

		for _, c := range configurations {
			switch format {
			case storageExportFormatCSV:
				fmt.Printf("%s,%s,%s,%d,%d,%s\n", c.Issuer, c.Username, c.Algorithm, c.Digits, c.Period, string(c.Secret))
			case storageExportFormatURI:
				fmt.Println(c.URI())
			}
		}

		if len(configurations) < limit {
			break
		}
	}

	return nil
}

func storageMigrateHistoryRunE(_ *cobra.Command, _ []string) (err error) {
	var (
		provider storage.Provider
		ctx      = context.Background()
	)

	provider = getStorageProvider()
	if provider == nil {
		return errNoStorageProvider
	}

	defer func() {
		_ = provider.Close()
	}()

	version, err := provider.SchemaVersion(ctx)
	if err != nil {
		return err
	}

	if version <= 0 {
		fmt.Println("No migration history is available for schemas that not version 1 or above.")
		return
	}

	migrations, err := provider.SchemaMigrationHistory(ctx)
	if err != nil {
		return err
	}

	if len(migrations) == 0 {
		return errors.New("no migration history found which may indicate a broken schema")
	}

	fmt.Printf("Migration History:\n\nID\tDate\t\t\t\tBefore\tAfter\tAuthelia Version\n")

	for _, m := range migrations {
		fmt.Printf("%d\t%s\t%d\t%d\t%s\n", m.ID, m.Applied.Format("2006-01-02 15:04:05 -0700"), m.Before, m.After, m.Version)
	}

	return nil
}

func newStorageMigrateListRunE(up bool) func(cmd *cobra.Command, args []string) (err error) {
	return func(cmd *cobra.Command, args []string) (err error) {
		var (
			provider     storage.Provider
			ctx          = context.Background()
			migrations   []models.SchemaMigration
			directionStr string
		)

		provider = getStorageProvider()

		defer func() {
			_ = provider.Close()
		}()

		if up {
			migrations, err = provider.SchemaMigrationsUp(ctx, 0)
			directionStr = "Up"
		} else {
			migrations, err = provider.SchemaMigrationsDown(ctx, 0)
			directionStr = "Down"
		}

		if err != nil && !errors.Is(err, storage.ErrNoAvailableMigrations) && !errors.Is(err, storage.ErrMigrateCurrentVersionSameAsTarget) {
			return err
		}

		if len(migrations) == 0 {
			fmt.Printf("Storage Schema Migration List (%s)\n\nNo Migrations Available\n", directionStr)
		} else {
			fmt.Printf("Storage Schema Migration List (%s)\n\nVersion\t\tDescription\n", directionStr)

			for _, migration := range migrations {
				fmt.Printf("%d\t\t%s\n", migration.Version, migration.Name)
			}
		}

		return nil
	}
}

func newStorageMigrationRunE(up bool) func(cmd *cobra.Command, args []string) (err error) {
	return func(cmd *cobra.Command, args []string) (err error) {
		var (
			provider storage.Provider
			ctx      = context.Background()
		)

		provider = getStorageProvider()

		defer func() {
			_ = provider.Close()
		}()

		target, err := cmd.Flags().GetInt("target")
		if err != nil {
			return err
		}

		switch {
		case up:
			switch cmd.Flags().Changed("target") {
			case true:
				return provider.SchemaMigrate(ctx, true, target)
			default:
				return provider.SchemaMigrate(ctx, true, storage.SchemaLatest)
			}
		default:
			pre1, err := cmd.Flags().GetBool("pre1")
			if err != nil {
				return err
			}

			if !cmd.Flags().Changed("target") && !pre1 {
				return errors.New("must set target")
			}

			if err = storageMigrateDownConfirmDestroy(cmd); err != nil {
				return err
			}

			switch {
			case pre1:
				return provider.SchemaMigrate(ctx, false, -1)
			default:
				return provider.SchemaMigrate(ctx, false, target)
			}
		}
	}
}

func storageMigrateDownConfirmDestroy(cmd *cobra.Command) (err error) {
	destroy, err := cmd.Flags().GetBool("destroy-data")
	if err != nil {
		return err
	}

	if !destroy {
		fmt.Printf("Schema Down Migrations may DESTROY data, type 'DESTROY' and press return to continue: ")

		var text string

		_, _ = fmt.Scanln(&text)

		if text != "DESTROY" {
			return errors.New("cancelling down migration due to user not accepting data destruction")
		}
	}

	return nil
}

func storageSchemaInfoRunE(_ *cobra.Command, _ []string) (err error) {
	var (
		provider   storage.Provider
		ctx        = context.Background()
		upgradeStr string
		tablesStr  string
	)

	provider = getStorageProvider()

	defer func() {
		_ = provider.Close()
	}()

	version, err := provider.SchemaVersion(ctx)
	if err != nil && err.Error() != "unknown schema state" {
		return err
	}

	tables, err := provider.SchemaTables(ctx)
	if err != nil {
		return err
	}

	if len(tables) == 0 {
		tablesStr = "N/A"
	} else {
		tablesStr = strings.Join(tables, ", ")
	}

	latest, err := provider.SchemaLatestVersion()
	if err != nil {
		return err
	}

	if latest > version {
		upgradeStr = fmt.Sprintf("yes - version %d", latest)
	} else {
		upgradeStr = "no"
	}

	var encryption string

	if err = provider.SchemaEncryptionCheckKey(ctx, false); err != nil {
		if errors.Is(err, storage.ErrSchemaEncryptionVersionUnsupported) {
			encryption = "unsupported (schema version)"
		} else {
			encryption = "invalid"
		}
	} else {
		encryption = "valid"
	}

	fmt.Printf("Schema Version: %s\nSchema Upgrade Available: %s\nSchema Tables: %s\nSchema Encryption Key: %s\n", storage.SchemaVersionToString(version), upgradeStr, tablesStr, encryption)

	return nil
}

func checkStorageSchemaUpToDate(ctx context.Context, provider storage.Provider) (err error) {
	version, err := provider.SchemaVersion(ctx)
	if err != nil {
		return err
	}

	latest, err := provider.SchemaLatestVersion()
	if err != nil {
		return err
	}

	if version != latest {
		return fmt.Errorf("schema is version %d which is outdated please migrate to version %d in order to use this command or use an older binary", version, latest)
	}

	return nil
}

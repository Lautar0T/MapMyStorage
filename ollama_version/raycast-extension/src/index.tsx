import {
  Action,
  ActionPanel,
  Form,
  Icon,
  List,
  Toast,
  getPreferenceValues,
  showToast,
  useNavigation,
} from "@raycast/api";
import { useCallback, useEffect, useMemo, useState } from "react";
import * as path from "path";
import { Entry, ScanConfig, expandHome, openInFinder, openInTerminal, revealInFinder, scanDirectory } from "./utils/exec";
import { formatBytes, formatDate, formatRatio, getCloudStatusIcon, getSizeColor } from "./utils/format";

interface Preferences {
  defaultRootPath: string;
  showHiddenFiles: boolean;
  excludePatterns: string;
  followSymlinks: boolean;
  maxScanDepth: string;
}

interface ScanArguments {
  path?: string;
}

interface RootFormProps {
  initialPath: string;
  onSubmit: (newRoot: string) => void;
}

function ChangeRootForm({ initialPath, onSubmit }: RootFormProps) {
  const [rootPath, setRootPath] = useState(initialPath);

  return (
    <Form
      actions={
        <ActionPanel>
          <Action.SubmitForm
            title="Use Root"
            icon={Icon.CheckCircle}
            onSubmit={() => {
              if (rootPath.trim()) {
                onSubmit(rootPath.trim());
              }
            }}
          />
        </ActionPanel>
      }
    >
      <Form.TextField id="root" title="Root Path" value={rootPath} onChange={setRootPath} />
    </Form>
  );
}

export default function Command(props: { arguments?: ScanArguments }) {
  const preferences = getPreferenceValues<Preferences>();
  const { push, pop } = useNavigation();

  const defaultRoot = expandHome(props.arguments?.path || preferences.defaultRootPath || "~");
  const [currentPath, setCurrentPath] = useState(defaultRoot);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchText, setSearchText] = useState("");
  const [cache, setCache] = useState<Record<string, Entry>>({});

  const configBase: Omit<ScanConfig, "rootPath"> = useMemo(
    () => ({
      showHidden: preferences.showHiddenFiles,
      excludePatterns: preferences.excludePatterns
        ? preferences.excludePatterns
            .split(",")
            .map((p) => p.trim())
            .filter(Boolean)
        : [],
      followSymlinks: preferences.followSymlinks,
      maxDepth: parseInt(preferences.maxScanDepth || "0", 10) || 0,
    }),
    [preferences],
  );

  const performScan = useCallback(
    async (targetPath: string, force = false) => {
      const fullPath = expandHome(targetPath);
      if (!force && cache[fullPath]) {
        setCurrentPath(fullPath);
        setError(null);
        return;
      }

      setIsLoading(true);
      setError(null);

      try {
        const rootEntry = await scanDirectory({ rootPath: fullPath, ...configBase });
        setCache((prev) => ({ ...prev, [fullPath]: rootEntry }));
        setCurrentPath(fullPath);
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setError(message);
        await showToast({ style: Toast.Style.Failure, title: "Scan Failed", message });
      } finally {
        setIsLoading(false);
      }
    },
    [cache, configBase],
  );

  useEffect(() => {
    void performScan(currentPath);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const rootEntry = cache[currentPath];
  const parentPath = useMemo(() => {
    const normalized = path.normalize(currentPath);
    const parent = path.dirname(normalized);
    if (parent === normalized) {
      return null;
    }
    return parent;
  }, [currentPath]);

  const entries = useMemo(() => {
    const children = [...(rootEntry?.children ?? [])];
    children.sort((a, b) => b.physical_size - a.physical_size);
    return children;
  }, [rootEntry]);

  const filtered = useMemo(() => {
    if (!searchText.trim()) {
      return entries;
    }
    const q = searchText.toLowerCase();
    return entries.filter((e) => e.name.toLowerCase().includes(q) || e.path.toLowerCase().includes(q));
  }, [entries, searchText]);

  const handleRefresh = () => {
    void performScan(currentPath, true);
  };

  const handleChangeRoot = () => {
    push(
      <ChangeRootForm
        initialPath={currentPath}
        onSubmit={(newRoot) => {
          pop();
          void performScan(newRoot, true);
        }}
      />,
    );
  };

  if (error) {
    return (
      <List>
        <List.EmptyView
          icon={Icon.XMarkCircle}
          title="Scan Error"
          description={error}
          actions={
            <ActionPanel>
              <Action title="Retry" icon={Icon.RotateClockwise} onAction={handleRefresh} />
              <Action title="Change Root" icon={Icon.Folder} onAction={handleChangeRoot} />
            </ActionPanel>
          }
        />
      </List>
    );
  }

  return (
    <List
      isLoading={isLoading}
      searchBarPlaceholder="Search files and folders..."
      searchText={searchText}
      onSearchTextChange={setSearchText}
      navigationTitle={`real-disk-map: ${currentPath}`}
      isShowingDetail={false}
    >
      {parentPath && (
        <List.Item
          key=".."
          title=".."
          subtitle="Go to parent directory"
          icon={Icon.ArrowUp}
          actions={
            <ActionPanel>
              <Action title="Go to Parent" icon={Icon.ArrowUp} onAction={() => void performScan(parentPath)} />
              <Action title="Refresh" icon={Icon.RotateClockwise} onAction={handleRefresh} />
            </ActionPanel>
          }
        />
      )}

      {filtered.map((entry) => {
        const ratio = entry.logical_size > 0 ? entry.physical_size / entry.logical_size : 1;
        const isOnlineOnly = entry.cloud_status === "online-only" || entry.physical_size === 0;

        return (
          <List.Item
            key={entry.path}
            title={entry.name}
            subtitle={`${entry.type} • ${formatDate(entry.mod_time)}`}
            icon={entry.type === "directory" ? Icon.Folder : entry.type === "symlink" ? Icon.Link : Icon.Document}
            accessories={[
              {
                tag: {
                  value: formatBytes(entry.physical_size),
                  color: isOnlineOnly ? "#6B7A8F" : getSizeColor(entry.physical_size),
                },
                tooltip: "Real allocated size",
              },
              { text: `L ${formatBytes(entry.logical_size)}`, tooltip: "Logical size" },
              { text: `R ${formatRatio(ratio)}`, tooltip: "Physical / logical ratio" },
              {
                text: entry.is_cloud
                  ? `${getCloudStatusIcon(entry.cloud_status)} ${entry.cloud_provider || "Cloud"}`
                  : "Local",
                tooltip: entry.cloud_status || "local",
              },
            ]}
            actions={
              <ActionPanel>
                <ActionPanel.Section>
                  {entry.type === "directory" && (
                    <Action title="Enter Directory" icon={Icon.ArrowRight} onAction={() => void performScan(entry.path)} />
                  )}
                  <Action title="Open in Finder" icon={Icon.Finder} onAction={() => void openInFinder(entry.path)} />
                  {entry.type !== "directory" && (
                    <Action title="Reveal File in Finder" icon={Icon.Eye} onAction={() => void revealInFinder(entry.path)} />
                  )}
                  <Action
                    title="Open in Terminal"
                    icon={Icon.Terminal}
                    onAction={() => void openInTerminal(entry.type === "directory" ? entry.path : path.dirname(entry.path))}
                  />
                </ActionPanel.Section>

                <ActionPanel.Section>
                  <Action.CopyToClipboard title="Copy Path" content={entry.path} shortcut={{ modifiers: ["cmd"], key: "c" }} />
                  <Action title="Refresh" icon={Icon.RotateClockwise} onAction={handleRefresh} shortcut={{ modifiers: ["cmd"], key: "r" }} />
                  <Action title="Change Root" icon={Icon.Folder} onAction={handleChangeRoot} shortcut={{ modifiers: ["cmd"], key: "o" }} />
                </ActionPanel.Section>
              </ActionPanel>
            }
          />
        );
      })}
    </List>
  );
}

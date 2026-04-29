// Formatting utilities for the Raycast extension

const UNITS = ["B", "KB", "MB", "GB", "TB", "PB"];

/**
 * Format bytes to human-readable string
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  if (bytes < 0) return "-" + formatBytes(-bytes);

  const exponent = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    UNITS.length - 1
  );

  const value = bytes / Math.pow(1024, exponent);
  const formatted = value < 10 ? value.toFixed(1) : Math.round(value).toString();

  return `${formatted} ${UNITS[exponent]}`;
}

/**
 * Format bytes to short string (no space between number and unit)
 */
export function formatBytesShort(bytes: number): string {
  if (bytes === 0) return "0B";
  if (bytes < 0) return "-" + formatBytesShort(-bytes);

  const exponent = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    UNITS.length - 1
  );

  const value = bytes / Math.pow(1024, exponent);
  const formatted = value < 10 ? value.toFixed(1) : Math.round(value).toString();

  return `${formatted}${UNITS[exponent]}`;
}

/**
 * Get a color based on size for visual indication
 */
export function getSizeColor(bytes: number): string {
  if (bytes > 10 * 1024 * 1024 * 1024) {
    // > 10 GB - Red
    return "#FF0000";
  } else if (bytes > 1024 * 1024 * 1024) {
    // > 1 GB - Orange
    return "#FF8800";
  } else if (bytes > 100 * 1024 * 1024) {
    // > 100 MB - Yellow
    return "#FFCC00";
  } else if (bytes > 0) {
    // > 0 - Green
    return "#00CC00";
  } else {
    // 0 - Gray
    return "#888888";
  }
}

/**
 * Format a size ratio as percentage
 */
export function formatRatio(ratio: number): string {
  if (!Number.isFinite(ratio) || ratio <= 0) return "0%";
  const pct = ratio * 100;
  if (pct > 9999) return "9999%+";
  if (pct < 1) return `${pct.toFixed(1)}%`;
  return `${Math.round(pct)}%`;
}

/**
 * Format a date string
 */
export function formatDate(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffDays === 0) {
    return "Today";
  } else if (diffDays === 1) {
    return "Yesterday";
  } else if (diffDays < 7) {
    return `${diffDays} days ago`;
  } else if (diffDays < 30) {
    return `${Math.floor(diffDays / 7)} weeks ago`;
  } else {
    return date.toLocaleDateString();
  }
}

/**
 * Format a path to show only the last components
 */
export function formatPath(path: string, maxLength: number = 50): string {
  if (path.length <= maxLength) return path;

  const parts = path.split("/");
  let result = path;

  while (result.length > maxLength && parts.length > 2) {
    parts.shift();
    result = ".../" + parts.join("/");
  }

  return result;
}

/**
 * Get an icon for a file type
 */
export function getFileIcon(type: string, isCloud: boolean): string {
  if (isCloud) return "☁️";

  switch (type) {
    case "directory":
      return "📁";
    case "symlink":
      return "🔗";
    case "file":
      return "📄";
    default:
      return "❓";
  }
}

/**
 * Get a status indicator for cloud files
 */
export function getCloudStatusIcon(status?: string): string {
  switch (status) {
    case "online-only":
      return "◯"; // Empty circle
    case "local":
      return "◉"; // Filled circle
    case "synced":
      return "✓"; // Checkmark
    case "downloading":
      return "↓"; // Download arrow
    default:
      return "◐"; // Half circle
  }
}

/**
 * Truncate a string to a maximum length
 */
export function truncate(str: string, maxLength: number): string {
  if (str.length <= maxLength) return str;
  return str.slice(0, maxLength - 3) + "...";
}

/**
 * Format number with thousands separator
 */
export function formatNumber(num: number): string {
  return num.toLocaleString();
}

/**
 * Calculate percentage
 */
export function calculatePercentage(part: number, total: number): string {
  if (total === 0) return "0%";
  return `${Math.round((part / total) * 100)}%`;
}

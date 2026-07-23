import { Fragment } from "react";
import styles from "./JsonTree.module.css";

/**
 * A purpose-built recursive JSON renderer — not a generic unstyled <pre>
 * block, and not a general-purpose code editor. Built specifically so a
 * single field, at a known dot-path, can be marked linkable, highlighted,
 * or quieted — the mechanism the field-lineage interaction depends on.
 * Semantic structure: each nesting level is a <dl>, each field a <dt>/<dd>
 * pair, so the raw event reads as real structured data, not decoration.
 */
export function JsonTree({
  data,
  path = "",
  linkablePaths,
  highlightedPaths,
  dimOthers,
  onSelectPath,
}: {
  data: unknown;
  path?: string;
  linkablePaths: Set<string>;
  highlightedPaths: Set<string>;
  dimOthers: boolean;
  onSelectPath: (path: string) => void;
}) {
  if (data === null || data === undefined) {
    return <span className={styles.nullValue}>null</span>;
  }

  if (Array.isArray(data)) {
    if (data.length === 0) return <span className={styles.emptyValue}>[]</span>;
    return (
      <dl className={styles.list}>
        {data.map((item, index) => {
          const itemPath = `${path}[${index}]`;
          return (
            <Fragment key={itemPath}>
              <dt className={styles.key}>{index}</dt>
              <dd className={styles.value}>
                <JsonTree
                  data={item}
                  path={itemPath}
                  linkablePaths={linkablePaths}
                  highlightedPaths={highlightedPaths}
                  dimOthers={dimOthers}
                  onSelectPath={onSelectPath}
                />
              </dd>
            </Fragment>
          );
        })}
      </dl>
    );
  }

  if (typeof data === "object") {
    const entries = Object.entries(data as Record<string, unknown>);
    if (entries.length === 0) return <span className={styles.emptyValue}>{"{}"}</span>;
    return (
      <dl className={styles.list}>
        {entries.map(([key, value]) => {
          const childPath = path ? `${path}.${key}` : key;
          const isPrimitive = value === null || typeof value !== "object";
          return (
            <Fragment key={childPath}>
              <dt className={styles.key}>{key}</dt>
              <dd className={styles.value}>
                {isPrimitive ? (
                  <FieldLeaf
                    path={childPath}
                    value={value}
                    linkable={linkablePaths.has(childPath)}
                    highlighted={highlightedPaths.has(childPath)}
                    dimmed={dimOthers && !highlightedPaths.has(childPath)}
                    onSelect={onSelectPath}
                  />
                ) : (
                  <JsonTree
                    data={value}
                    path={childPath}
                    linkablePaths={linkablePaths}
                    highlightedPaths={highlightedPaths}
                    dimOthers={dimOthers}
                    onSelectPath={onSelectPath}
                  />
                )}
              </dd>
            </Fragment>
          );
        })}
      </dl>
    );
  }

  return <span className={styles.scalarValue}>{formatScalar(data)}</span>;
}

function FieldLeaf({
  path,
  value,
  linkable,
  highlighted,
  dimmed,
  onSelect,
}: {
  path: string;
  value: unknown;
  linkable: boolean;
  highlighted: boolean;
  dimmed: boolean;
  onSelect: (path: string) => void;
}) {
  const formatted = formatScalar(value);

  if (!linkable) {
    return (
      <span className={styles.scalarValue} data-dimmed={dimmed || undefined}>
        {formatted}
      </span>
    );
  }

  return (
    <button
      type="button"
      className={styles.linkableValue}
      data-highlighted={highlighted || undefined}
      data-dimmed={(dimmed && !highlighted) || undefined}
      data-lineage-path={path}
      onClick={() => onSelect(path)}
      aria-pressed={highlighted}
    >
      {formatted}
    </button>
  );
}

function formatScalar(value: unknown): string {
  if (typeof value === "string") return `"${value}"`;
  if (typeof value === "boolean") return value ? "true" : "false";
  return String(value);
}

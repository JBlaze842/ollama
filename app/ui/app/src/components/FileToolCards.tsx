import React from "react";

type FileSearchResult = {
  pattern: string;
  scope: string;
  matches: Array<{ path: string; type: string }>;
  truncated?: boolean;
};

type FileGrepResult = {
  query: string;
  scope: string;
  matches: Array<{ path: string; line: number; content: string }>;
  files_searched: number;
  files_skipped: number;
  truncated?: boolean;
};

type FileReadResult = {
  path: string;
  start_line: number;
  end_line: number;
  total_lines: number;
  truncated?: boolean;
  content: string;
};

type FilePatchResult = {
  path: string;
  edits: Array<{
    old_text: string;
    new_text: string;
    matches: number;
    replace_all?: boolean;
  }>;
  total_replacements: number;
};

type FileMoveResult = {
  source_path: string;
  destination_path: string;
  source_type: string;
  overwritten?: boolean;
  destination_exists?: boolean;
};

type FileMkdirResult = {
  path: string;
  created?: boolean;
  already_exists?: boolean;
};

type FileWriteResult = FileReadResult & {
  created?: boolean;
  overwritten?: boolean;
  bytes?: number;
};

function isObject(value: unknown): value is Record<string, any> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function isFileSearchResult(value: unknown): value is FileSearchResult {
  return (
    isObject(value) &&
    typeof value.pattern === "string" &&
    Array.isArray(value.matches)
  );
}

function isFileGrepResult(value: unknown): value is FileGrepResult {
  return (
    isObject(value) &&
    typeof value.query === "string" &&
    Array.isArray(value.matches)
  );
}

function isFileReadResult(value: unknown): value is FileReadResult {
  return (
    isObject(value) &&
    typeof value.path === "string" &&
    typeof value.start_line === "number" &&
    typeof value.content === "string"
  );
}

function isFilePatchResult(value: unknown): value is FilePatchResult {
  return (
    isObject(value) &&
    typeof value.path === "string" &&
    Array.isArray(value.edits)
  );
}

function isFileMoveResult(value: unknown): value is FileMoveResult {
  return (
    isObject(value) &&
    typeof value.source_path === "string" &&
    typeof value.destination_path === "string"
  );
}

function isFileMkdirResult(value: unknown): value is FileMkdirResult {
  return isObject(value) && typeof value.path === "string";
}

function isFileWriteResult(value: unknown): value is FileWriteResult {
  return isFileReadResult(value);
}

function Section({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-xl border border-neutral-200 bg-neutral-50 p-3 text-neutral-800 dark:border-neutral-700 dark:bg-neutral-900/60 dark:text-neutral-200">
      <div className="mb-3 flex flex-col gap-1">
        <div className="text-sm font-medium">{title}</div>
        {subtitle ? (
          <div className="font-mono text-xs text-neutral-500 dark:text-neutral-400">
            {subtitle}
          </div>
        ) : null}
      </div>
      {children}
    </div>
  );
}

function LinePreview({ result }: { result: FileReadResult | FileWriteResult }) {
  const lines =
    result.content.length > 0 ? result.content.split("\n") : ([] as string[]);

  return (
    <div className="overflow-auto rounded-lg border border-neutral-200 bg-white dark:border-neutral-700 dark:bg-neutral-950">
      <div className="min-w-full">
        {lines.length === 0 ? (
          <div className="px-3 py-2 text-xs text-neutral-500 dark:text-neutral-400">
            Empty file
          </div>
        ) : (
          lines.map((line, index) => {
            const lineNumber = result.start_line + index;
            return (
              <div
                key={`${lineNumber}-${index}`}
                className="flex font-mono text-xs"
              >
                <div className="w-14 flex-shrink-0 border-r border-neutral-200 px-2 py-1 text-right text-neutral-500 dark:border-neutral-700 dark:text-neutral-400">
                  {lineNumber}
                </div>
                <div className="min-w-0 flex-1 px-3 py-1 whitespace-pre-wrap break-all">
                  {line}
                </div>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}

function FileSearchCard({ result }: { result: FileSearchResult }) {
  return (
    <Section
      title={`Workspace search for "${result.pattern}"`}
      subtitle={result.scope || "."}
    >
      <div className="space-y-2 text-xs">
        {result.matches.length === 0 ? (
          <div className="text-neutral-500 dark:text-neutral-400">
            No matches.
          </div>
        ) : (
          result.matches.map((match, index) => (
            <div
              key={`${match.path}-${index}`}
              className="flex items-start gap-2 rounded-lg bg-white px-3 py-2 dark:bg-neutral-950"
            >
              <span className="rounded-full border border-neutral-200 px-2 py-0.5 font-mono text-[10px] uppercase tracking-wide text-neutral-500 dark:border-neutral-700 dark:text-neutral-400">
                {match.type}
              </span>
              <span className="font-mono break-all">{match.path}</span>
            </div>
          ))
        )}
        {result.truncated ? (
          <div className="text-neutral-500 dark:text-neutral-400">
            Results truncated.
          </div>
        ) : null}
      </div>
    </Section>
  );
}

function FileGrepCard({ result }: { result: FileGrepResult }) {
  return (
    <Section
      title={`Text matches for "${result.query}"`}
      subtitle={`${result.scope || "."} • ${result.files_searched} searched • ${result.files_skipped} skipped`}
    >
      <div className="space-y-2 text-xs">
        {result.matches.length === 0 ? (
          <div className="text-neutral-500 dark:text-neutral-400">
            No matches.
          </div>
        ) : (
          result.matches.map((match, index) => (
            <div
              key={`${match.path}-${match.line}-${index}`}
              className="rounded-lg bg-white px-3 py-2 dark:bg-neutral-950"
            >
              <div className="mb-1 font-mono text-[11px] text-neutral-500 dark:text-neutral-400">
                {match.path}:{match.line}
              </div>
              <div className="font-mono whitespace-pre-wrap break-all">
                {match.content}
              </div>
            </div>
          ))
        )}
        {result.truncated ? (
          <div className="text-neutral-500 dark:text-neutral-400">
            Results truncated.
          </div>
        ) : null}
      </div>
    </Section>
  );
}

function FileReadCard({
  result,
  title,
}: {
  result: FileReadResult | FileWriteResult;
  title: string;
}) {
  return (
    <Section
      title={title}
      subtitle={`${result.path} • lines ${result.start_line}-${result.end_line} of ${result.total_lines}`}
    >
      <LinePreview result={result} />
      {result.truncated ? (
        <div className="mt-2 text-xs text-neutral-500 dark:text-neutral-400">
          Preview truncated.
        </div>
      ) : null}
    </Section>
  );
}

function FilePatchCard({
  result,
  title,
}: {
  result: FilePatchResult;
  title: string;
}) {
  return (
    <Section
      title={title}
      subtitle={`${result.path} • ${result.total_replacements} replacement(s)`}
    >
      <div className="space-y-3">
        {result.edits.map((edit, index) => (
          <div key={index} className="rounded-lg bg-white p-3 dark:bg-neutral-950">
            <div className="mb-2 text-xs text-neutral-500 dark:text-neutral-400">
              Edit {index + 1} • {edit.matches} replacement(s)
              {edit.replace_all ? " • replace all" : ""}
            </div>
            <div className="grid gap-2 md:grid-cols-2">
              <div>
                <div className="mb-1 text-[11px] uppercase tracking-wide text-neutral-500 dark:text-neutral-400">
                  Old
                </div>
                <pre className="overflow-auto rounded border border-neutral-200 bg-neutral-50 p-2 text-xs whitespace-pre-wrap dark:border-neutral-700 dark:bg-neutral-900">
                  <code>{edit.old_text}</code>
                </pre>
              </div>
              <div>
                <div className="mb-1 text-[11px] uppercase tracking-wide text-neutral-500 dark:text-neutral-400">
                  New
                </div>
                <pre className="overflow-auto rounded border border-neutral-200 bg-neutral-50 p-2 text-xs whitespace-pre-wrap dark:border-neutral-700 dark:bg-neutral-900">
                  <code>{edit.new_text}</code>
                </pre>
              </div>
            </div>
          </div>
        ))}
      </div>
    </Section>
  );
}

function FileMoveCard({ result }: { result: FileMoveResult }) {
  return (
    <Section
      title={`Move ${result.source_type === "dir" ? "directory" : "file"}`}
      subtitle={`${result.source_path} → ${result.destination_path}`}
    >
      <div className="text-xs text-neutral-600 dark:text-neutral-300">
        {result.overwritten
          ? "Existing destination will be replaced."
          : "Destination does not exist yet."}
      </div>
    </Section>
  );
}

function FileMkdirCard({ result }: { result: FileMkdirResult }) {
  return (
    <Section title="Create directory" subtitle={result.path}>
      <div className="text-xs text-neutral-600 dark:text-neutral-300">
        {result.already_exists ? "Directory already exists." : "Directory will be created."}
      </div>
    </Section>
  );
}

export function FileToolStructuredResult({
  toolName,
  result,
}: {
  toolName?: string;
  result: unknown;
}) {
  switch (toolName) {
    case "fs_search":
      return isFileSearchResult(result) ? <FileSearchCard result={result} /> : null;
    case "fs_grep":
      return isFileGrepResult(result) ? <FileGrepCard result={result} /> : null;
    case "fs_read":
      return isFileReadResult(result) ? (
        <FileReadCard result={result} title="Read file" />
      ) : null;
    case "fs_patch":
      return isFilePatchResult(result) ? (
        <FilePatchCard result={result} title="Applied patch" />
      ) : null;
    default:
      return null;
  }
}

export function FileToolStructuredPreview({
  toolName,
  preview,
}: {
  toolName?: string;
  preview: unknown;
}) {
  switch (toolName) {
    case "fs_write":
      return isFileWriteResult(preview) ? (
        <FileReadCard result={preview} title="Proposed file content" />
      ) : null;
    case "fs_patch":
      return isFilePatchResult(preview) ? (
        <FilePatchCard result={preview} title="Proposed patch" />
      ) : null;
    case "fs_move":
      return isFileMoveResult(preview) ? <FileMoveCard result={preview} /> : null;
    case "fs_mkdir":
      return isFileMkdirResult(preview) ? <FileMkdirCard result={preview} /> : null;
    default:
      return null;
  }
}

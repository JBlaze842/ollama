/* FileToolsButton Component */
import { forwardRef } from "react";

interface ButtonProps {
  isVisible?: boolean;
  isActive: boolean;
  isDisabled?: boolean;
  disabledReason?: string;
  onToggle: () => void;
}

export const FileToolsButton = forwardRef<HTMLButtonElement, ButtonProps>(
  function FileToolsButton(
    { isVisible, isActive, isDisabled, disabledReason, onToggle },
    ref,
  ) {
    if (!isVisible) return null;

    const title = isDisabled
      ? disabledReason || "Workspace file tools are unavailable"
      : isActive
        ? "Disable workspace file tools"
        : "Enable workspace file tools";

    return (
      <button
        ref={ref}
        type="button"
        title={title}
        disabled={isDisabled}
        onClick={onToggle}
        className={`select-none flex items-center justify-center rounded-full h-9 w-9 bg-white dark:bg-neutral-700 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all whitespace-nowrap border border-transparent ${
          isDisabled ? "cursor-not-allowed opacity-40" : "cursor-pointer"
        } ${
          isActive
            ? "text-[rgba(0,115,255,1)] dark:text-[rgba(70,155,255,1)]"
            : "text-neutral-500 dark:text-neutral-400"
        }`}
      >
        <svg
          className="h-5 w-5"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.8"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M4.5 7.5 12 3l7.5 4.5v9L12 21l-7.5-4.5v-9Z"
          />
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M9 10.5h6M9 13.5h4.5"
          />
        </svg>
      </button>
    );
  },
);
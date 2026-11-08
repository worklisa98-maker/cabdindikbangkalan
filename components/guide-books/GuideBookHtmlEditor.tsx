"use client";

import { useEffect, useRef, useState } from "react";
import { EditorContent, useEditor } from "@tiptap/react";
import Image from "@tiptap/extension-image";
import Link from "@tiptap/extension-link";
import TextAlign from "@tiptap/extension-text-align";
import Underline from "@tiptap/extension-underline";
import StarterKit from "@tiptap/starter-kit";

import { cn } from "@/lib/utils";

type GuideBookHtmlEditorProps = {
  value: string;
  onChange: (value: string) => void;
  onUploadImage: (file: File) => Promise<string>;
  disabled?: boolean;
  allowImages?: boolean;
};

type ToolbarButtonProps = {
  label: string;
  onClick: () => void;
  active?: boolean;
  disabled?: boolean;
};

function ToolbarButton({
  label,
  onClick,
  active = false,
  disabled = false,
}: ToolbarButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "rounded-md border px-2 py-1 text-xs font-medium transition",
        active
          ? "border-zinc-900 bg-zinc-900 text-white"
          : "border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50",
        disabled && "cursor-not-allowed opacity-60"
      )}
    >
      {label}
    </button>
  );
}

export function GuideBookHtmlEditor({
  value,
  onChange,
  onUploadImage,
  disabled = false,
  allowImages = true,
}: GuideBookHtmlEditorProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [uploadError, setUploadError] = useState("");

  const editor = useEditor({
    immediatelyRender: false,
    extensions: [
      StarterKit,
      Underline,
      TextAlign.configure({
        types: ["heading", "paragraph"],
      }),
      Image,
      Link.configure({
        openOnClick: false,
        autolink: true,
        HTMLAttributes: {
          rel: "noopener noreferrer",
          target: "_blank",
        },
      }),
    ],
    content: value || "<p></p>",
    onUpdate({ editor: currentEditor }) {
      onChange(currentEditor.getHTML());
    },
    editorProps: {
      attributes: {
        class:
          "min-h-56 w-full rounded-b-md p-3 outline-none [&_blockquote]:border-l-4 [&_blockquote]:border-zinc-300 [&_blockquote]:pl-3 [&_h1]:text-2xl [&_h1]:font-semibold [&_h2]:text-xl [&_h2]:font-semibold [&_h3]:text-lg [&_h3]:font-semibold [&_img]:my-2 [&_img]:h-auto [&_img]:max-w-full [&_ol]:list-decimal [&_ol]:pl-6 [&_p]:my-2 [&_pre]:overflow-x-auto [&_pre]:rounded-md [&_pre]:bg-zinc-900 [&_pre]:p-3 [&_pre]:text-zinc-100 [&_ul]:list-disc [&_ul]:pl-6",
      },
    },
  });

  useEffect(() => {
    if (!editor) return;
    const current = editor.getHTML();
    const next = value || "<p></p>";
    if (current !== next) {
      editor.commands.setContent(next, false);
    }
  }, [editor, value]);

  const handleSetLink = () => {
    if (!editor || disabled) return;
    const previous = editor.getAttributes("link").href ?? "";
    const raw = window.prompt("Masukkan URL link", previous);
    if (raw === null) return;

    const trimmed = raw.trim();
    if (!trimmed) {
      editor.chain().focus().extendMarkRange("link").unsetLink().run();
      return;
    }

    let href = trimmed;
    if (!/^https?:\/\//i.test(href) && !href.startsWith("/")) {
      href = `https://${href}`;
    }

    editor
      .chain()
      .focus()
      .extendMarkRange("link")
      .setLink({ href, target: "_blank", rel: "noopener noreferrer" })
      .run();
  };

  const handleUploadImage = async (
    event: React.ChangeEvent<HTMLInputElement>
  ) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file || !editor) return;

    setUploadError("");
    setIsUploading(true);
    try {
      const url = await onUploadImage(file);
      editor.chain().focus().setImage({ src: url, alt: file.name }).run();
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Upload gambar gagal.";
      setUploadError(message || "Upload gambar gagal.");
    } finally {
      setIsUploading(false);
    }
  };

  return (
    <div className="rounded-md border border-zinc-200 bg-white">
      <div className="flex flex-wrap gap-2 border-b border-zinc-200 p-2">
        <ToolbarButton
          label="B"
          active={editor?.isActive("bold")}
          disabled={disabled || !editor}
          onClick={() => editor?.chain().focus().toggleBold().run()}
        />
        <ToolbarButton
          label="I"
          active={editor?.isActive("italic")}
          disabled={disabled || !editor}
          onClick={() => editor?.chain().focus().toggleItalic().run()}
        />
        <ToolbarButton
          label="U"
          active={editor?.isActive("underline")}
          disabled={disabled || !editor}
          onClick={() => editor?.chain().focus().toggleUnderline().run()}
        />
        <ToolbarButton
          label="P"
          active={editor?.isActive("paragraph")}
          disabled={disabled || !editor}
          onClick={() => editor?.chain().focus().setParagraph().run()}
        />
        <ToolbarButton
          label="List"
          active={editor?.isActive("bulletList")}
          disabled={disabled || !editor}
          onClick={() => editor?.chain().focus().toggleBulletList().run()}
        />
        <ToolbarButton
          label="1."
          active={editor?.isActive("orderedList")}
          disabled={disabled || !editor}
          onClick={() => editor?.chain().focus().toggleOrderedList().run()}
        />
        <ToolbarButton
          label="Kiri"
          active={editor?.isActive({ textAlign: "left" })}
          disabled={disabled || !editor}
          onClick={() => editor?.chain().focus().setTextAlign("left").run()}
        />
        <ToolbarButton
          label="Tengah"
          active={editor?.isActive({ textAlign: "center" })}
          disabled={disabled || !editor}
          onClick={() => editor?.chain().focus().setTextAlign("center").run()}
        />
        <ToolbarButton
          label="Kanan"
          active={editor?.isActive({ textAlign: "right" })}
          disabled={disabled || !editor}
          onClick={() => editor?.chain().focus().setTextAlign("right").run()}
        />
        <ToolbarButton
          label="Indent"
          disabled={disabled || !editor}
          onClick={() => editor?.chain().focus().sinkListItem("listItem").run()}
        />
        <ToolbarButton
          label="Outdent"
          disabled={disabled || !editor}
          onClick={() => editor?.chain().focus().liftListItem("listItem").run()}
        />
        <ToolbarButton
          label="Link"
          active={editor?.isActive("link")}
          disabled={disabled || !editor}
          onClick={handleSetLink}
        />
        {allowImages ? (
          <>
            <ToolbarButton
              label={isUploading ? "Upload..." : "Gambar"}
              disabled={disabled || !editor || isUploading}
              onClick={() => fileInputRef.current?.click()}
            />
            <input
              ref={fileInputRef}
              type="file"
              accept="image/png,image/jpeg,image/webp"
              className="hidden"
              onChange={handleUploadImage}
            />
          </>
        ) : null}
      </div>
      <EditorContent editor={editor} />
      {uploadError ? (
        <p className="border-t border-zinc-200 px-3 py-2 text-xs text-red-600">
          {uploadError}
        </p>
      ) : null}
    </div>
  );
}

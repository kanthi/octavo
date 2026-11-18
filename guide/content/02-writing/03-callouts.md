# Callouts

A callout is a box that stands out from the text: a note, a tip, a warning. Write a block quote whose first line is a marker:

```markdown
> [!NOTE]
> A note gives the reader extra context.
```

There are five kinds:

> [!NOTE]
> Extra context the reader may want.

> [!TIP]
> A better or quicker way to do something.

> [!IMPORTANT]
> Something the reader must not skip.

> [!WARNING]
> Something that can go wrong.

> [!CAUTION]
> Something that can do harm, such as losing data.

The marker must be alone on the first line, in capitals: `[!NOTE]`, `[!TIP]`, `[!IMPORTANT]`, `[!WARNING]` or `[!CAUTION]`. The rest of the callout is ordinary Markdown, so it can hold lists, code and links.

This is the same way of writing callouts that GitHub uses, so the chapters also read well on GitHub.

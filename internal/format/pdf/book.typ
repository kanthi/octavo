// Octavo's book layout for Typst. "octavo" (paper, margins, font and the
// rest of "format: pdf:" in octavo.yml) is defined before this.

#set document(title: octavo.title, author: if octavo.author == "" { () } else { octavo.author })
#set page(..octavo.paper, margin: octavo.margin)
#set text(font: octavo.font, size: octavo.size, lang: octavo.lang, hyphenate: octavo.justify)
#set par(justify: octavo.justify, leading: 0.72em, spacing: 1.15em)

#let soft = luma(85)

// Code. Typst already sets code at 0.8 of the text around it, as the
// browser engine does for blocks; text in a line is set a little larger.
// JetBrains Mono, without its programming ligatures ("!=" stays "!=").
#show raw: set text(font: ("JetBrains Mono", "DejaVu Sans Mono"), ligatures: false, features: (calt: 0))
#show raw.where(block: true): it => block(
  width: 100%, fill: luma(247), stroke: 0.5pt + luma(222), radius: 3pt,
  inset: (x: 0.9em, y: 0.75em), breakable: true,
  { set par(justify: false); it },
)
// A highlight, not a box: a box cannot break across lines.
#show raw.where(block: false): it => highlight(fill: luma(238), extent: 1.2pt, radius: 1.5pt,
  top-edge: "ascender", bottom-edge: "descender", text(size: 1.05em, it))

// Links and text-only parts.
#show link: set text(fill: rgb("#1f4fa8"))
#show list: set par(justify: false)
#show enum: set par(justify: false)
#set list(indent: 0.4em, body-indent: 0.6em)
#set enum(indent: 0.4em, body-indent: 0.6em)

// Headings inside a chapter.
#set heading(numbering: none)
#show heading: set text(hyphenate: false)
#show heading: set par(justify: false)
#show heading.where(level: 2): set text(size: 1.42em)
#show heading.where(level: 3): set text(size: 1.15em)
#show heading.where(level: 4): set text(size: 1.0em)
#show heading.where(level: 5): set text(size: 1.0em, style: "italic")
#show heading.where(level: 2): set block(above: 2.1em, below: 0.9em)
#show heading.where(level: 3): set block(above: 1.7em, below: 0.7em)
#show heading.where(level: 4): set block(above: 1.4em, below: 0.6em)

// A chapter's title, and a part's.
#show heading.where(level: 1): it => {
  if it.supplement == [Part] {
    text(size: 2.4em, weight: 700, it.body)
  } else {
    set text(size: 2.15em, weight: 700, hyphenate: false)
    set par(justify: false, leading: 0.45em)
    block(width: 100%, below: 1.6em, inset: (bottom: 0.55em), stroke: (bottom: 0.75pt + black), it.body)
  }
}

// Tables.
#show table: set par(justify: false)
#show table: set text(size: 0.88em)
#set table(
  inset: (x: 0.5em, y: 0.42em),
  stroke: (x, y) => if y == 0 { (bottom: 0.7pt + black) } else { (bottom: 0.3pt + luma(215)) },
)
#show table.cell.where(y: 0): set text(weight: 700)
#let octavo-table(..args) = block(width: 100%, breakable: true, table(..args))

// Pieces chapters use.
#let octavo-rule() = align(center, line(length: 30%, stroke: 0.5pt + luma(180)))
#let octavo-quote(body) = block(
  inset: (left: 1em, y: 0.15em), stroke: (left: 2pt + luma(205)),
  text(fill: luma(70), body),
)
#let octavo-kbd(body) = box(stroke: 0.5pt + luma(170), radius: 2pt, inset: (x: 3pt), outset: (y: 2pt), text(size: 0.85em, body))
#let octavo-box(done) = box(baseline: 15%, rect(width: 0.8em, height: 0.8em, radius: 1.5pt, stroke: 0.6pt + soft,
  inset: 0pt, if done { align(center + horizon, text(size: 0.7em, "✓")) }))
#let octavo-caption(body) = align(center, text(size: 0.88em, fill: luma(60), body))
#let octavo-img(path, width: auto) = layout(size => {
  if width != auto { image(path, width: width) } else {
    let natural = measure(image(path)).width
    if natural > size.width { image(path, width: 100%) } else { image(path) }
  }
})
#let octavo-figure(body, caption) = block(width: 100%, breakable: false, above: 1.4em, below: 1.4em, {
  align(center, body)
  if caption != [] { v(0.3em); octavo-caption(caption) }
})
#let callout-colors = (
  note: rgb("#2456c9"), tip: rgb("#1a7f4b"), warning: rgb("#b06a00"),
  important: rgb("#7a3fc4"), caution: rgb("#c0362c"),
)
#let callout(kind, title, body) = {
  let c = callout-colors.at(kind, default: callout-colors.note)
  block(width: 100%, breakable: true, fill: c.lighten(94%), stroke: (left: 2.5pt + c),
    radius: (right: 3pt), inset: (left: 1em, right: 0.9em, y: 0.75em), {
      set par(justify: false)
      if title != [] { block(below: 0.6em, text(weight: 700, fill: c.darken(10%), title)) }
      body
    })
}

// The running header: the book on the left, the chapter on the right, from
// the page after a chapter starts. Each chapter leaves its names in a
// metadata element labelled octavo-chapter.
#let running-header = context {
  let pg = here().page()
  if query(<octavo-chapter>).any(m => m.location().page() == pg) { return none }
  let before = query(selector(<octavo-chapter>).before(here()))
  if before.len() == 0 { return none }
  let m = before.last().value
  set text(size: 8.5pt, style: "italic", fill: soft)
  block(inset: (bottom: 4mm), grid(columns: (1fr, auto), column-gutter: 1em, m.left, m.right))
}
#let page-number = context align(center, text(size: 9pt, fill: soft, counter(page).display()))
#set page(
  header: if octavo.headers { running-header } else { none },
  footer: if octavo.page-numbers { page-number } else { none },
  numbering: "i",
)

#let title-page(title, author, date, description) = page(header: none, footer: none, {
  set par(justify: false)
  align(center + horizon, {
    text(size: 2.9em, weight: 700, title)
    v(0.2em)
    line(length: 3.5em, stroke: 1.2pt)
    v(1.2em)
    if author != "" { text(size: 1.25em, author); linebreak() }
    if date != "" { text(fill: soft, date) }
    if description != "" { v(2.5em); pad(x: 12%, text(style: "italic", fill: luma(70), description)) }
  })
})

#let contents-heading() = {
  pagebreak(weak: true)
  v(2.4em)
  heading(level: 1, outlined: false)[Contents]
}

// A line in the contents: number, title, dots, page.
#let toc-line(num, title, target, depth) = context {
  let loc = locate(target)
  let pg = counter(page).at(loc).first()
  show link: set text(fill: black)
  block(above: 0.45em, below: 0.45em, inset: (left: depth * 1.4em), link(loc, grid(
    columns: (if num == none { 0em } else { 2.2em }, 1fr, auto),
    text(fill: soft, if num == none { [] } else { num }),
    [#title #box(width: 1fr, repeat(gap: 0.25em)[.])],
    if octavo.page-numbers { h(0.4em) + str(pg) },
  )))
}
#let toc-group(title, depth) = block(above: 1.1em, below: 0.4em, inset: (left: depth * 1.4em), sticky: true,
  text(weight: if depth == 0 { 700 } else { 600 }, style: if depth == 0 { "normal" } else { "italic" }, title))

#let part-page(n, title) = page(header: none, footer: none, numbering: "1", {
  set par(justify: false)
  align(center + horizon, {
    text(size: 0.85em, tracking: 0.2em, fill: soft, upper[Part #n])
    v(1em)
    heading(level: 1, supplement: [Part], title)
  })
})

#let chapter-start(kicker, left, right) = {
  pagebreak(weak: true)
  counter(footnote).update(0)
  v(2.6em)
  if kicker != none { text(size: 0.86em, tracking: 0.2em, fill: soft, upper(kicker)); v(0.4em) }
  [#metadata((left: left, right: right)) <octavo-chapter>]
}

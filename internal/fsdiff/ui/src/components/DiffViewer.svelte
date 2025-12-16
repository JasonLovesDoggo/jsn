<script lang="ts">
  interface Props {
    diff: string;
  }

  let { diff }: Props = $props();

  interface DiffLine {
    type: 'header' | 'add' | 'remove' | 'context';
    text: string;
    lineNum?: number;
  }

  function parseDiff(diffText: string): DiffLine[] {
    const lines = diffText.split('\n');
    const result: DiffLine[] = [];
    let lineNum = 0;

    for (const line of lines) {
      if (line.startsWith('@@')) {
        result.push({ type: 'header', text: line });
        const match = line.match(/@@ -\d+(?:,\d+)? \+(\d+)/);
        if (match) {
          lineNum = parseInt(match[1], 10);
        }
      } else if (line.startsWith('+')) {
        result.push({ type: 'add', text: line.slice(1), lineNum });
        lineNum++;
      } else if (line.startsWith('-')) {
        result.push({ type: 'remove', text: line.slice(1) });
      } else if (line.startsWith(' ')) {
        result.push({ type: 'context', text: line.slice(1), lineNum });
        lineNum++;
      } else if (line.length === 0 && result.length > 0) {
        result.push({ type: 'context', text: '', lineNum });
        lineNum++;
      }
    }

    return result;
  }

  const parsedLines = $derived(parseDiff(diff));
</script>

<div class="font-mono text-sm">
  {#each parsedLines as line, i (i)}
    {#if line.type === 'header'}
      <div class="px-3 py-1 bg-accent/20 text-accent border-y border-bg-3">
        {line.text}
      </div>
    {:else}
      <div
        class="flex hover:bg-bg-2 {line.type === 'add'
          ? 'bg-added/10'
          : line.type === 'remove'
            ? 'bg-removed/10'
            : ''}"
      >
        <span
          class="w-12 px-2 py-0.5 text-right select-none border-r border-bg-3 shrink-0
                 {line.type === 'add'
                   ? 'text-added bg-added/20'
                   : line.type === 'remove'
                     ? 'text-removed bg-removed/20'
                     : 'text-fg-3'}"
        >
          {line.type === 'add' ? '+' : line.type === 'remove' ? '-' : ''}
        </span>
        <pre
          class="px-3 py-0.5 whitespace-pre overflow-x-auto flex-1
                {line.type === 'add'
                  ? 'text-added'
                  : line.type === 'remove'
                    ? 'text-removed'
                    : 'text-fg-1'}">{line.text}</pre>
      </div>
    {/if}
  {/each}
</div>

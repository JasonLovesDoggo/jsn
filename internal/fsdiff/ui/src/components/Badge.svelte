<script lang="ts">
  type BadgeType = 'suid' | 'sgid' | 'exec' | 'cap' | 'imm';

  interface Props {
    type: BadgeType;
  }

  let { type }: Props = $props();

  const labels: Record<BadgeType, string> = {
    suid: 'SUID',
    sgid: 'SGID',
    exec: 'EXEC',
    cap: 'CAP',
    imm: 'IMM',
  };

  const colors: Record<BadgeType, string> = {
    suid: 'bg-suid/20 text-suid',
    sgid: 'bg-suid/20 text-suid',
    exec: 'bg-exec/20 text-exec',
    cap: 'bg-cap/20 text-cap',
    imm: 'bg-immutable/20 text-immutable',
  };
</script>

<span class="px-1.5 py-0.5 text-[10px] font-medium rounded {colors[type]}">
  {labels[type]}
</span>

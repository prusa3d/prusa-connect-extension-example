<script lang="ts">
  import { describeFault, type FaultText } from '@prusa/connect-ui/i18n';
  import {
    Card,
    CardContent,
    FaultAlert,
    PageHeader,
    Skeleton,
  } from '@prusa/connect-ui/primitives';

  import { type Hello, helloOn } from './hello';
  import { heldHost } from './host';

  // A page takes no props: the route is the whole context.
  const host = heldHost();
  const t = host.t;

  let hello = $state<Hello | undefined>();
  let fault = $state<FaultText | undefined>();

  // One read when the page opens. host.signal ends it when the person leaves.
  $effect(() => {
    helloOn(host)
      .read(host.signal)
      .then((answer) => (hello = answer))
      .catch((thrown: unknown) => {
        if (!host.signal.aborted) fault = describeFault(thrown);
      });
  });
</script>

<!-- The shape of the app's own screens: the heading, then the cards. The
     host draws no heading over a module's page, so this one is the page's. -->
<div class="flex flex-col gap-4">
  <PageHeader title={t('hello.page.title')} />

  {#if fault}
    <FaultAlert {fault} />
  {:else}
    <Card>
      <CardContent>
        <!-- Every text a page draws comes from this module's own catalogue,
             through the `t` the host bound to this module's namespace: a
             literal here is refused by lint. -->
        <p class="mb-4 text-sm text-muted-foreground">{t('hello.page.body')}</p>
        {#if hello}
          <dl class="grid w-fit grid-cols-2 gap-x-8 gap-y-2 text-sm">
            <dt class="text-muted-foreground">{t('hello.page.message')}</dt>
            <dd>{hello.message}</dd>
            <dt class="text-muted-foreground">{t('hello.page.principal')}</dt>
            <dd class="font-mono">{hello.principal}</dd>
          </dl>
        {:else}
          <Skeleton class="h-16 w-full" />
        {/if}
      </CardContent>
    </Card>
  {/if}
</div>

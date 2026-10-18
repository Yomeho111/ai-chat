<script setup lang='ts'>
import { computed, ref } from 'vue'
import { NDropdown } from 'naive-ui'
import AvatarComponent from './Avatar.vue'
import TextComponent from './Text.vue'
import { SvgIcon } from '@/components/common'
import { copyText } from '@/utils/format'
import { useIconRender } from '@/hooks/useIconRender'
import { t } from '@/locales'
import { useBasicLayout } from '@/hooks/useBasicLayout'

interface Props {
  dateTime?: string
  source?: Chat.AnswerSource
  tokenUsage?: Chat.TokenUsage
  tokenUsageComplete?: boolean
  text?: string
  inversion?: boolean
  error?: boolean
  loading?: boolean
}

interface Emit {
  (ev: 'regenerate'): void
  (ev: 'delete'): void
}

const props = defineProps<Props>()

const emit = defineEmits<Emit>()

const { isMobile } = useBasicLayout()

const { iconRender } = useIconRender()

const textRef = ref<HTMLElement>()

const asRawText = ref(props.inversion)

const messageRef = ref<HTMLElement>()

const sourceBadge = computed(() => {
  switch (props.source) {
    case 'vector_db':
      return {
        label: t('chat.sourceVectorDB'),
        icon: 'ri:database-2-fill',
        classes: 'border-emerald-500 bg-emerald-100 text-emerald-800 dark:border-emerald-400 dark:bg-emerald-900 dark:text-emerald-100',
      }
    case 'llm_api':
      return {
        label: t('chat.sourceLLMAPI'),
        icon: 'ri:robot-fill',
        classes: 'border-blue-500 bg-blue-100 text-blue-800 dark:border-blue-400 dark:bg-blue-900 dark:text-blue-100',
      }
    case 'sensitive_filter':
      return {
        label: t('chat.sourceSensitiveFilter'),
        icon: 'ri:shield-check-fill',
        classes: 'border-amber-500 bg-amber-100 text-amber-800 dark:border-amber-400 dark:bg-amber-900 dark:text-amber-100',
      }
    default:
      return {
        label: t(props.loading ? 'chat.sourcePending' : 'chat.sourceUnknown'),
        icon: 'ri:question-fill',
        classes: 'border-neutral-400 bg-neutral-100 text-neutral-700 dark:border-neutral-500 dark:bg-neutral-800 dark:text-neutral-200',
      }
  }
})

const options = computed(() => {
  const common = [
    {
      label: t('chat.copy'),
      key: 'copyText',
      icon: iconRender({ icon: 'ri:file-copy-2-line' }),
    },
    {
      label: t('common.delete'),
      key: 'delete',
      icon: iconRender({ icon: 'ri:delete-bin-line' }),
    },
  ]

  if (!props.inversion) {
    common.unshift({
      label: asRawText.value ? t('chat.preview') : t('chat.showRawText'),
      key: 'toggleRenderType',
      icon: iconRender({ icon: asRawText.value ? 'ic:outline-code-off' : 'ic:outline-code' }),
    })
  }

  return common
})

function handleSelect(key: 'copyText' | 'delete' | 'toggleRenderType') {
  switch (key) {
    case 'copyText':
      copyText({ text: props.text ?? '' })
      return
    case 'toggleRenderType':
      asRawText.value = !asRawText.value
      return
    case 'delete':
      emit('delete')
  }
}

function handleRegenerate() {
  messageRef.value?.scrollIntoView()
  emit('regenerate')
}
</script>

<template>
  <div
    ref="messageRef"
    class="flex w-full mb-6 overflow-hidden"
    :class="[{ 'flex-row-reverse': inversion }]"
  >
    <div
      class="flex items-center justify-center flex-shrink-0 h-8 overflow-hidden rounded-full basis-8"
      :class="[inversion ? 'ml-2' : 'mr-2']"
    >
      <AvatarComponent :image="inversion" />
    </div>
    <div class="overflow-hidden text-sm " :class="[inversion ? 'items-end' : 'items-start']">
      <p class="flex flex-wrap items-center gap-2 text-xs text-[#b4bbc4]" :class="[inversion ? 'justify-end' : 'justify-start']">
        <span>{{ dateTime }}</span>
        <span
          v-if="!inversion"
          class="inline-flex items-center gap-1 px-2 py-0.5 font-semibold border rounded-full"
          :class="sourceBadge.classes"
        >
          <SvgIcon :icon="sourceBadge.icon" aria-hidden="true" />
          {{ sourceBadge.label }}
        </span>
      </p>
      <div
        class="flex items-end gap-1 mt-2"
        :class="[inversion ? 'flex-row-reverse' : 'flex-row']"
      >
        <TextComponent
          ref="textRef"
          :inversion="inversion"
          :error="error"
          :text="text"
          :loading="loading"
          :as-raw-text="asRawText"
        />
        <div class="flex flex-col">
          <button
            v-if="!inversion"
            class="mb-2 transition text-neutral-300 hover:text-neutral-800 dark:hover:text-neutral-300"
            @click="handleRegenerate"
          >
            <SvgIcon icon="ri:restart-line" />
          </button>
          <NDropdown
            :trigger="isMobile ? 'click' : 'hover'"
            :placement="!inversion ? 'right' : 'left'"
            :options="options"
            @select="handleSelect"
          >
            <button class="transition text-neutral-300 hover:text-neutral-800 dark:hover:text-neutral-200">
              <SvgIcon icon="ri:more-2-fill" />
            </button>
          </NDropdown>
        </div>
      </div>
      <p v-if="!inversion" class="flex flex-wrap gap-x-3 gap-y-1 mt-1 text-xs text-neutral-500 dark:text-neutral-400">
        <span>{{ t('chat.tokensConsumed') }}: {{ tokenUsage?.consumedTokens.toLocaleString() ?? '--' }}</span>
        <span>{{ t('chat.tokensSaved') }}: {{ tokenUsage?.savedTokens.toLocaleString() ?? '--' }}</span>
        <span v-if="!tokenUsage || tokenUsageComplete === false">
          {{ loading ? t('chat.tokenUsagePending') : t('chat.tokenUsageUnavailable') }}
        </span>
      </p>
    </div>
  </div>
</template>

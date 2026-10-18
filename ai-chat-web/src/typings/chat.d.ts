declare namespace Chat {

	type AnswerSource = 'vector_db' | 'llm_api' | 'sensitive_filter'

	interface TokenUsage {
		consumedTokens: number
		savedTokens: number
	}

	interface Chat {
		dateTime: string
		text: string
		source?: AnswerSource
		tokenUsage?: TokenUsage
		tokenUsageComplete?: boolean
		inversion?: boolean
		error?: boolean
		loading?: boolean
		conversationOptions?: ConversationRequest | null
		requestOptions: { prompt: string; options?: ConversationRequest | null }
	}

	interface History {
		title: string
		isEdit: boolean
		uuid: number
	}

	interface ChatState {
		active: number | null
		usingContext: boolean;
		history: History[]
		chat: { uuid: number; data: Chat[] }[]
	}

	interface ConversationRequest {
		conversationId?: string
		parentMessageId?: string
	}

	interface ConversationResponse {
		conversationId: string
		source?: AnswerSource
		tokenUsage?: TokenUsage
		detail: {
			choices: { finish_reason?: string; finishReason?: string; index: number; logprobs: any; text: string }[]
			created: number
			id: string
			model: string
			object: string
			usage: { completion_tokens: number; prompt_tokens: number; total_tokens: number }
		}
		id: string
		parentMessageId: string
		role: string
		text: string
	}
}

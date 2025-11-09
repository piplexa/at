CREATE TABLE scheduled_tasks (
    id BIGSERIAL PRIMARY KEY,
    execute_at TIMESTAMPTZ NOT NULL,
    task_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'cancelled')),
    attempts INT DEFAULT 0,
    max_attempts INT DEFAULT 3,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- Индекс для быстрого поиска заданий к выполнению
CREATE INDEX idx_pending_tasks 
ON scheduled_tasks(execute_at, status) 
WHERE status IN ('pending', 'processing');

-- Индекс для мониторинга и статистики
CREATE INDEX idx_status_type 
ON scheduled_tasks(status, task_type);

-- Индекс для поиска зависших заданий
CREATE INDEX idx_processing_timeout 
ON scheduled_tasks(updated_at) 
WHERE status = 'processing';

-- Триггер для автообновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_scheduled_tasks_updated_at
BEFORE UPDATE ON scheduled_tasks
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();
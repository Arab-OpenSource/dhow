import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { Badge } from '@/Core/Ui/Badge';
import { Card } from '@/Core/Ui/Card';
import { Field } from '@/Core/Ui/Field';
import { Input } from '@/Core/Ui/Input';
import { Row, Rows } from '@/Core/Ui/Row';
import { Summary } from '@/Core/Ui/Summary';

describe('Card', () => {
  it('is flat: a thin line, no gradient, a header icon, a title and an end hint', () => {
    const { container } = render(
      <Card title="Sign-in methods" icon="key" end="3 devices">
        <p>Body</p>
      </Card>,
    );
    const card = container.querySelector<HTMLElement>('section');
    if (!card) throw new Error('the card did not render');
    expect(card).toHaveClass('border-line', 'bg-surface', 'rounded-md');
    expect(screen.getByRole('heading', { level: 3 })).toHaveClass('text-title-card');
    expect(card.querySelector('header')).toHaveClass('px-card');
    expect(screen.getByRole('heading', { level: 3, name: 'Sign-in methods' })).toBeInTheDocument();
    expect(within(card).getByText('3 devices')).toBeInTheDocument();
    expect(card.querySelector('header svg')).toHaveAttribute('aria-hidden', 'true');
  });

  it('renders without a header', () => {
    render(
      <Card>
        <p>Only body</p>
      </Card>,
    );
    expect(screen.queryByRole('heading')).not.toBeInTheDocument();
    expect(screen.getByText('Only body')).toBeInTheDocument();
  });
});

describe('Row', () => {
  it('shows a title, a sub-line and an end slot as a list item', () => {
    render(
      <Rows label="Checks">
        <Row title="Database" sub="Answers in 4 ms" end={<Badge>Running</Badge>} />
        <Row title="Migrations" />
      </Rows>,
    );
    const list = screen.getByRole('list', { name: 'Checks' });
    const [first, second] = within(list).getAllByRole('listitem');
    expect(first).toHaveTextContent('DatabaseAnswers in 4 msRunning');
    expect(first).toHaveClass('first:border-t-0');
    expect(second).toHaveClass('border-t', 'border-line');
  });

  it('becomes a single button with a chevron when it can be opened', async () => {
    const onSelect = vi.fn();
    render(
      <Rows>
        <Row title="Database" onSelect={onSelect} />
      </Rows>,
    );
    await userEvent.click(screen.getByRole('button', { name: 'Database' }));
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it('isolates a technical value so it reads left to right inside Arabic', () => {
    render(
      <Rows>
        <Row title="ci-github-actions" mono />
      </Rows>,
    );
    const value = screen.getByText('ci-github-actions');
    expect(value.tagName).toBe('BDI');
    expect(value).toHaveAttribute('dir', 'ltr');
  });
});

describe('Summary', () => {
  it('names the state with a title and a sub-line, tinted by tone', () => {
    const { container, rerender } = render(<Summary icon="shield" title="Your account is well protected" sub="Two passkeys" />);
    expect(container.firstElementChild).toHaveAttribute('data-tone', 'ok');
    expect(container.firstElementChild).toHaveClass('border-ok-line', 'bg-ok-wash');
    expect(screen.getByText('Two passkeys')).toBeInTheDocument();
    rerender(<Summary tone="bad" icon="alert" title="A check is failing" />);
    expect(container.firstElementChild).toHaveClass('border-bad-line', 'bg-bad-wash');
  });
});

describe('Badge', () => {
  it('has a tone for every status, with a line, a wash and a foreground', () => {
    render(
      <>
        <Badge tone="info">Building</Badge>
        <Badge tone="queue">Queued</Badge>
      </>,
    );
    expect(screen.getByText('Building')).toHaveClass('border-info-line', 'bg-info-wash', 'text-info-fg');
    expect(screen.getByText('Queued')).toHaveClass('border-queue-line', 'bg-queue-wash', 'text-queue-fg');
    expect(screen.getByText('Queued')).toHaveClass('text-caption', 'border');
  });

  it('has ok, warn, bad and mute tones and always carries a word', () => {
    render(
      <>
        <Badge>Running</Badge>
        <Badge tone="warn">Pending</Badge>
        <Badge tone="bad" icon="alert">
          Failed
        </Badge>
        <Badge tone="mute">Current</Badge>
      </>,
    );
    expect(screen.getByText('Running')).toHaveClass('bg-ok-wash', 'text-ok-fg');
    expect(screen.getByText('Pending')).toHaveClass('bg-warn-wash');
    expect(screen.getByText('Failed')).toHaveClass('bg-bad-wash');
    expect(screen.getByText('Failed').querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
    expect(screen.getByText('Current')).toHaveClass('bg-wash');
  });
});

describe('Field and Input', () => {
  it('names the input by its label', () => {
    render(
      <Field label="Theme name">
        <Input />
      </Field>,
    );
    const input = screen.getByLabelText('Theme name');
    expect(input).toHaveClass('h-input', 'rounded-input', 'border-line-2');
    expect(input).not.toHaveAttribute('aria-invalid');
  });

  it('links an error to the input and marks it invalid', () => {
    render(
      <Field label="Theme name" error="Give the theme a name.">
        <Input />
      </Field>,
    );
    const input = screen.getByLabelText('Theme name');
    expect(input).toHaveAttribute('aria-invalid', 'true');
    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('Give the theme a name.');
    expect(input).toHaveAttribute('aria-describedby', alert.id);
  });

  it('has a compact size for use inside rows', () => {
    render(<Input compact aria-label="Hex" />);
    expect(screen.getByLabelText('Hex')).toHaveClass('h-btn-sm');
  });
});

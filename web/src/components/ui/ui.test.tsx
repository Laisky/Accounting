import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Button, Card, ConfirmDialog, EmptyState, FormField, Notice, Sheet } from './index';

describe('Button', () => {
  it('defaults to type=button and applies the variant class', () => {
    render(<Button variant="danger">Delete</Button>);
    const button = screen.getByRole('button', { name: 'Delete' });
    expect(button).toHaveAttribute('type', 'button');
    expect(button).toHaveClass('ui-button', 'ui-button-danger');
  });
});

describe('Card', () => {
  it('renders a raised surface container with extra classes', () => {
    render(
      <Card as="section" className="rep-card" aria-label="Summary">
        body
      </Card>,
    );
    const card = screen.getByLabelText('Summary');
    expect(card.tagName).toBe('SECTION');
    expect(card).toHaveClass('ui-card', 'rep-card');
  });
});

describe('EmptyState', () => {
  it('renders a primary action that fires onAction', () => {
    const onAction = vi.fn();
    render(
      <EmptyState
        title="No books yet"
        description="Create one to start"
        actionLabel="Create book"
        onAction={onAction}
      />,
    );
    expect(screen.getByText('No books yet')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Create book' }));
    expect(onAction).toHaveBeenCalledTimes(1);
  });
});

describe('FormField', () => {
  it('wires the label, hint, and error to the control via ids', () => {
    render(
      <FormField label="Amount" hint="In cents" error="Required">
        {({ id, describedBy, invalid }) => <input id={id} aria-describedby={describedBy} aria-invalid={invalid} />}
      </FormField>,
    );
    const input = screen.getByLabelText('Amount');
    const describedBy = input.getAttribute('aria-describedby') ?? '';
    expect(describedBy).toMatch(/-hint/);
    expect(describedBy).toMatch(/-error/);
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByRole('alert')).toHaveTextContent('Required');
  });
});

describe('Notice', () => {
  it('uses role=alert for errors and role=status otherwise', () => {
    const { rerender } = render(<Notice tone="error">Boom</Notice>);
    expect(screen.getByRole('alert')).toHaveTextContent('Boom');
    rerender(<Notice tone="success">Saved</Notice>);
    expect(screen.getByRole('status')).toHaveTextContent('Saved');
  });
});

describe('ConfirmDialog', () => {
  it('fires onConfirm from the destructive action and onCancel from cancel', () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(
      <ConfirmDialog
        open
        title="Delete this entry?"
        description="It will be removed."
        confirmLabel="Delete entry"
        cancelLabel="Cancel"
        destructive
        onConfirm={onConfirm}
        onCancel={onCancel}
      />,
    );
    expect(screen.getByRole('dialog', { name: 'Delete this entry?' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Delete entry' }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getAllByRole('button', { name: 'Cancel' })[0]!);
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});

describe('Sheet', () => {
  it('renders nothing when closed', () => {
    render(
      <Sheet open={false} title="Edit" onClose={() => {}}>
        <button type="button">Inside</button>
      </Sheet>,
    );
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('traps focus, closes on Escape, and restores focus on close', () => {
    const onClose = vi.fn();
    function Harness() {
      return (
        <div>
          <button type="button">opener</button>
          <Sheet open title="Edit entry" onClose={onClose}>
            <button type="button">first</button>
            <button type="button">last</button>
          </Sheet>
        </div>
      );
    }
    render(<Harness />);

    const dialog = screen.getByRole('dialog', { name: 'Edit entry' });
    expect(dialog).toHaveAttribute('aria-modal', 'true');
    // Focus moved into the sheet (the close button is the first focusable).
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Close' }));

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('closes when the backdrop is clicked but not the sheet body', () => {
    const onClose = vi.fn();
    render(
      <Sheet open title="Edit" onClose={onClose}>
        <p>body</p>
      </Sheet>,
    );
    fireEvent.mouseDown(screen.getByText('body'));
    expect(onClose).not.toHaveBeenCalled();
    fireEvent.mouseDown(document.querySelector('.ui-sheet-backdrop') as HTMLElement);
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});


-- version 1

DROP SCHEMA IF EXISTS midgard_agg CASCADE;
CREATE SCHEMA midgard_agg;

CREATE VIEW midgard_agg.pending_adds AS
SELECT *
FROM pending_liquidity_events AS p
WHERE pending_type = 'add'
    AND NOT EXISTS(
        -- Filter out pending liquidity which was already added
        SELECT *
        FROM stake_events AS s
        WHERE
            p.rune_addr = s.rune_addr
            AND p.pool = s.pool
            AND p.block_timestamp <= s.block_timestamp)
    AND NOT EXISTS(
        -- Filter out pending liquidity which was withdrawn without adding
        SELECT *
        FROM pending_liquidity_events AS pw
        WHERE
            pw.pending_type = 'withdraw'
            AND p.rune_addr = pw.rune_addr
            AND p.pool = pw.pool
            AND p.block_timestamp <= pw.block_timestamp);

CREATE TABLE midgard_agg.watermarks (
    materialized_table varchar PRIMARY KEY,
    watermark bigint NOT NULL
);

CREATE FUNCTION midgard_agg.watermark(t varchar) RETURNS bigint
LANGUAGE SQL STABLE AS $$
    SELECT watermark FROM midgard_agg.watermarks
    WHERE materialized_table = t;
$$;

CREATE PROCEDURE midgard_agg.refresh_watermarked_view(t varchar, w_new bigint)
LANGUAGE plpgsql AS $BODY$
DECLARE
    w_old bigint;
BEGIN
    SELECT watermark FROM midgard_agg.watermarks WHERE materialized_table = t
        FOR UPDATE INTO w_old;
    IF w_new <= w_old THEN
        RAISE WARNING 'Updating % into past: % -> %', t, w_old, w_new;
        RETURN;
    END IF;
    EXECUTE format($$
        INSERT INTO midgard_agg.%1$I_materialized
        SELECT * from midgard_agg.%1$I
            WHERE $1 <= block_timestamp AND block_timestamp < $2
    $$, t) USING w_old, w_new;
    UPDATE midgard_agg.watermarks SET watermark = w_new WHERE materialized_table = t;
END
$BODY$;

-------------------------------------------------------------------------------
-- Actions
-------------------------------------------------------------------------------

-- TODO(huginn): there types and functions should go into the main DDL,
-- so we can use them unqualified.

CREATE TYPE midgard_agg.coin_rec as (asset text, amount bigint);

CREATE FUNCTION midgard_agg.non_null_array(VARIADIC elems text[])
RETURNS text[] LANGUAGE SQL IMMUTABLE AS $$
    SELECT array_remove(elems, NULL)
$$;

CREATE FUNCTION midgard_agg.coins(VARIADIC coins midgard_agg.coin_rec[])
RETURNS jsonb[] LANGUAGE SQL IMMUTABLE AS $$
    SELECT array_agg(jsonb_build_object('asset', asset, 'amount', amount))
    FROM unnest(coins)
    WHERE amount > 0
$$;


CREATE FUNCTION midgard_agg.mktransaction(
    txid text,
    address text,
    VARIADIC coins midgard_agg.coin_rec[]
) RETURNS jsonb LANGUAGE SQL IMMUTABLE AS $$
    SELECT jsonb_build_object(
        'txID', txid,
        'address', address,
        'coins', midgard_agg.coins(VARIADIC coins)
        )
$$;

-- TODO(huginn): better condition in WHERE
CREATE FUNCTION midgard_agg.transaction_list(VARIADIC txs jsonb[])
RETURNS jsonb LANGUAGE SQL IMMUTABLE AS $$
    SELECT COALESCE(jsonb_agg(tx), '[]' :: jsonb)
    FROM unnest(txs) t(tx)
    WHERE tx->>'coins' <> 'null';
$$;

--
-- Main table and its indices
--

CREATE TABLE midgard_agg.actions (
    height              bigint NOT NULL,
    block_timestamp     bigint NOT NULL,
    -- TODO(huginn): rename
    type                text NOT NULL,
    main_ref            text,
    addresses           text[] NOT NULL,
    transactions        text[] NOT NULL,
    assets              text[] NOT NULL,
    pools               text[],
    ins                 jsonb NOT NULL,
    outs                jsonb NOT NULL,
    fees                jsonb NOT NULL,
    meta                jsonb
);

-- TODO(huginn): should it be a hypertable? Measure both ways and decide!

CREATE INDEX ON midgard_agg.actions (block_timestamp);
CREATE INDEX ON midgard_agg.actions (type, block_timestamp);
CREATE INDEX ON midgard_agg.actions (main_ref, block_timestamp);

CREATE INDEX ON midgard_agg.actions USING gin (addresses);
CREATE INDEX ON midgard_agg.actions USING gin (transactions);
CREATE INDEX ON midgard_agg.actions USING gin (assets);
CREATE INDEX ON midgard_agg.actions USING gin ((meta-> 'affiliateAddress'));

--
-- Basic VIEWs that build actions
--

CREATE VIEW midgard_agg.switch_actions AS
    SELECT
        0 :: bigint as height,
        block_timestamp,
        'switch' as type,
        tx :: text as main_ref,
        ARRAY[from_addr, to_addr] :: text[] as addresses,
        midgard_agg.non_null_array(tx) as transactions,
        ARRAY[burn_asset, 'THOR.RUNE'] :: text[] as assets,
        NULL :: text[] as pools,
        jsonb_build_array(midgard_agg.mktransaction(tx, from_addr, (burn_asset, burn_e8))) as ins,
        jsonb_build_array(midgard_agg.mktransaction(NULL, to_addr, ('THOR.RUNE', burn_e8))) as outs,
        jsonb_build_array() as fees,
        NULL :: jsonb as meta
    FROM switch_events;

CREATE VIEW midgard_agg.refund_actions AS
    SELECT
        0 :: bigint as height,
        block_timestamp,
        'refund' as type,
        tx :: text as main_ref,
        ARRAY[from_addr, to_addr] :: text[] as addresses,
        ARRAY[tx] :: text[] as transactions,
        midgard_agg.non_null_array(asset, asset_2nd) as assets,
        NULL :: text[] as pools,
        jsonb_build_array(midgard_agg.mktransaction(tx, from_addr, (asset, asset_e8))) as ins,
        jsonb_build_array() as outs,
        jsonb_build_array() as fees,
        jsonb_build_object('reason', reason) as meta
    FROM refund_events;

CREATE VIEW midgard_agg.donate_actions AS
    SELECT
        0 :: bigint as height,
        block_timestamp,
        'donate' as type,
        tx :: text as main_ref,
        ARRAY[from_addr, to_addr] :: text[] as addresses,
        ARRAY[tx] :: text[] as transactions,
        CASE WHEN rune_e8 > 0 THEN ARRAY[asset, 'THOR.RUNE']
            ELSE ARRAY[asset] END :: text[] as assets,
        ARRAY[pool] :: text[] as pools,
        jsonb_build_array(midgard_agg.mktransaction(tx, from_addr, (asset, asset_e8),
            ('THOR.RUNE', rune_e8))) as ins,
        jsonb_build_array() as outs,
        jsonb_build_array() as fees,
        NULL :: jsonb as meta
    FROM add_events;

CREATE VIEW midgard_agg.withdraw_actions AS
    SELECT
        0 :: bigint as height,
        block_timestamp,
        'withdraw' as type,
        tx :: text as main_ref,
        ARRAY[from_addr, to_addr] :: text[] as addresses,
        ARRAY[tx] :: text[] as transactions,
        ARRAY[pool] :: text[] as assets,
        ARRAY[pool] :: text[] as pools,
        jsonb_build_array(midgard_agg.mktransaction(tx, from_addr, (asset, asset_e8))) as ins,
        jsonb_build_array() as outs,
        jsonb_build_array() as fees,
        jsonb_build_object(
            'asymmetry', asymmetry,
            'basisPoints', basis_points,
            'impermanentLossProtection', imp_loss_protection_e8,
            'liquidityUnits', -stake_units,
            'emitAssetE8', emit_asset_e8,
            'emitRuneE8', emit_rune_e8
            ) as meta
    FROM unstake_events;

-- TODO(huginn): use _direction for join
CREATE VIEW midgard_agg.swap_actions AS
    -- Single swap (unique txid)
    SELECT
        0 :: bigint as height,
        block_timestamp,
        'swap' as type,
        tx :: text as main_ref,
        ARRAY[from_addr, to_addr] :: text[] as addresses,
        ARRAY[tx] :: text[] as transactions,
        ARRAY[from_asset, to_asset] :: text[] as assets,
        ARRAY[pool] :: text[] as pools,
        jsonb_build_array(midgard_agg.mktransaction(tx, from_addr, (from_asset, from_e8))) as ins,
        jsonb_build_array() as outs,
        jsonb_build_array() as fees,
        jsonb_build_object(
            'swapSingle', TRUE,
            'liquidityFee', liq_fee_in_rune_e8,
            'swapTarget', to_e8_min,
            'swapSlip', swap_slip_bp,
            'affiliateFee', CASE
                WHEN SUBSTRING(memo FROM ':.*:.*:.*:(.*):.*') = to_addr THEN NULL
                ELSE SUBSTRING(memo FROM ':.*:.*:.*:.*:(\d{1,5})(:|$)')::int
            END,
            'affiliateAddress', CASE
                WHEN SUBSTRING(memo FROM ':.*:.*:.*:(.*):.*') = to_addr THEN NULL
                ELSE SUBSTRING(memo FROM ':.*:.*:.*:(.+):.*')
            END
            ) as meta
    FROM swap_events AS single_swaps
    WHERE NOT EXISTS (
        SELECT tx FROM swap_events
        WHERE block_timestamp = single_swaps.block_timestamp AND tx = single_swaps.tx
            AND from_asset <> single_swaps.from_asset
    )
    UNION ALL
    -- Double swap (same txid in different pools)
    SELECT
        0 :: bigint as height,
        swap_in.block_timestamp,
        'swap' as type,
        swap_in.tx :: text as main_ref,
        ARRAY[swap_in.from_addr, swap_in.to_addr] :: text[] as addresses,
        ARRAY[swap_in.tx] :: text[] as transactions,
        ARRAY[swap_in.from_asset, swap_out.to_asset] :: text[] as assets,
        CASE WHEN swap_in.pool <> swap_out.pool THEN ARRAY[swap_in.pool, swap_out.pool]
            ELSE ARRAY[swap_in.pool] END :: text[] as pools,
        jsonb_build_array(midgard_agg.mktransaction(swap_in.tx, swap_in.from_addr,
            (swap_in.from_asset, swap_in.from_e8))) as ins,
        jsonb_build_array() as outs,
        jsonb_build_array() as fees,
        jsonb_build_object(
            'swapSingle', FALSE,
            'liquidityFee', swap_in.liq_fee_in_rune_e8 + swap_out.liq_fee_in_rune_e8,
            'swapTarget', swap_out.to_e8_min,
            'swapSlip', swap_in.swap_slip_BP + swap_out.swap_slip_BP
                - swap_in.swap_slip_BP*swap_out.swap_slip_BP/10000,
            'affiliateFee', CASE
                WHEN SUBSTRING(swap_in.memo FROM ':.*:.*:.*:(.*):.*') = swap_in.to_addr THEN NULL
                ELSE SUBSTRING(swap_in.memo FROM ':.*:.*:.*:.*:(\d{1,5})(:|$)')::int
            END,
            'affiliateAddress', CASE
                WHEN SUBSTRING(swap_in.memo FROM ':.*:.*:.*:(.*):.*') = swap_in.to_addr THEN NULL
                ELSE SUBSTRING(swap_in.memo FROM ':.*:.*:.*:(.+):.*')
            END
            ) as meta
    FROM swap_events AS swap_in
    INNER JOIN swap_events AS swap_out
    ON swap_in.tx = swap_out.tx AND swap_in.block_timestamp = swap_out.block_timestamp
    WHERE swap_in.from_asset <> swap_out.to_asset AND swap_in.to_asset = 'THOR.RUNE'
        AND swap_out.from_asset = 'THOR.RUNE'
    ;

CREATE VIEW midgard_agg.addliquidity_actions AS
    SELECT
        0 :: bigint as height,
        block_timestamp,
        'addLiquidity' as type,
        NULL :: text as main_ref,
        midgard_agg.non_null_array(rune_addr, asset_addr) as addresses,
        midgard_agg.non_null_array(rune_tx, asset_tx) as transactions,
        ARRAY[pool, 'THOR.RUNE'] :: text[] as assets,
        ARRAY[pool] :: text[] as pools,
        midgard_agg.transaction_list(
            midgard_agg.mktransaction(rune_tx, rune_addr, ('THOR.RUNE', rune_e8)),
            midgard_agg.mktransaction(asset_tx, asset_addr, (pool, asset_e8))
            ) as ins,
        jsonb_build_array() as outs,
        jsonb_build_array() as fees,
        jsonb_build_object(
            'status', 'success',
            'liquidityUnits', stake_units
            ) as meta
    FROM stake_events
    UNION ALL
    -- Pending `add`s will be removed when not pending anymore
    SELECT
        0 :: bigint as height,
        block_timestamp,
        'addLiquidity' as type,
        'PL:' || rune_addr || ':' || pool :: text as main_ref,
        midgard_agg.non_null_array(rune_addr, asset_addr) as addresses,
        midgard_agg.non_null_array(rune_tx, asset_tx) as transactions,
        ARRAY[pool, 'THOR.RUNE'] :: text[] as assets,
        ARRAY[pool] :: text[] as pools,
        midgard_agg.transaction_list(
            midgard_agg.mktransaction(rune_tx, rune_addr, ('THOR.RUNE', rune_e8)),
            midgard_agg.mktransaction(asset_tx, asset_addr, (pool, asset_e8))
            ) as ins,
        jsonb_build_array() as outs,
        jsonb_build_array() as fees,
        jsonb_build_object('status', 'pending') as meta
    FROM pending_liquidity_events
    WHERE pending_type = 'add'
    ;

--
-- Procedures for updating actions
--

CREATE PROCEDURE midgard_agg.insert_actions(t1 bigint, t2 bigint)
LANGUAGE plpgsql AS $BODY$
BEGIN
    EXECUTE $$ INSERT INTO midgard_agg.actions
    SELECT * FROM midgard_agg.switch_actions
        WHERE $1 <= block_timestamp AND block_timestamp < $2 $$ USING t1, t2;

    EXECUTE $$ INSERT INTO midgard_agg.actions
    SELECT * FROM midgard_agg.refund_actions
        WHERE $1 <= block_timestamp AND block_timestamp < $2 $$ USING t1, t2;

    EXECUTE $$ INSERT INTO midgard_agg.actions
    SELECT * FROM midgard_agg.donate_actions
        WHERE $1 <= block_timestamp AND block_timestamp < $2 $$ USING t1, t2;

    EXECUTE $$ INSERT INTO midgard_agg.actions
    SELECT * FROM midgard_agg.withdraw_actions
        WHERE $1 <= block_timestamp AND block_timestamp < $2 $$ USING t1, t2;

    EXECUTE $$ INSERT INTO midgard_agg.actions
    SELECT * FROM midgard_agg.swap_actions
        WHERE $1 <= block_timestamp AND block_timestamp < $2 $$ USING t1, t2;

    EXECUTE $$ INSERT INTO midgard_agg.actions
    SELECT * FROM midgard_agg.addliquidity_actions
        WHERE $1 <= block_timestamp AND block_timestamp < $2 $$ USING t1, t2;
END
$BODY$;

CREATE PROCEDURE midgard_agg.set_actions_height(t1 bigint, t2 bigint)
LANGUAGE SQL AS $BODY$
    UPDATE midgard_agg.actions AS a
    SET height = bl.height
    FROM block_log AS bl
    WHERE bl.timestamp = a.block_timestamp AND t1 <= a.block_timestamp AND a.block_timestamp < t2;
$BODY$;

-- TODO(muninn): Check the pending logic regarding nil rune address
CREATE PROCEDURE midgard_agg.trim_pending_actions(t1 bigint, t2 bigint)
LANGUAGE SQL AS $BODY$
    DELETE FROM midgard_agg.actions AS a
    USING stake_events AS s
    WHERE
        t1 <= s.block_timestamp AND s.block_timestamp < t2
        AND a.block_timestamp <= s.block_timestamp
        AND a.main_ref = 'PL:' || s.rune_addr || ':' || s.pool;

    DELETE FROM midgard_agg.actions AS a
    USING pending_liquidity_events AS pw
    WHERE
        t1 <= pw.block_timestamp AND pw.block_timestamp < t2
        AND a.block_timestamp <= pw.block_timestamp
        AND pw.pending_type = 'withdraw'
        AND a.main_ref = 'PL:' || pw.rune_addr || ':' || pw.pool;
$BODY$;

-- TODO(huginn): Remove duplicates from these lists?
CREATE PROCEDURE midgard_agg.actions_add_outbounds(t1 bigint, t2 bigint)
LANGUAGE SQL AS $BODY$
    UPDATE midgard_agg.actions AS a
    SET
        addresses = a.addresses || o.froms || o.tos,
        transactions = a.transactions || array_remove(o.transactions, NULL),
        assets = a.assets || o.assets,
        outs = a.outs || o.outs
    FROM (
        SELECT
            in_tx,
            array_agg(from_addr :: text) AS froms,
            array_agg(to_addr :: text) AS tos,
            array_agg(tx :: text) AS transactions,
            array_agg(asset :: text) AS assets,
            jsonb_agg(midgard_agg.mktransaction(tx, to_addr, (asset, asset_e8))) AS outs
        FROM outbound_events
        WHERE t1 <= block_timestamp AND block_timestamp < t2
        GROUP BY in_tx
        ) AS o
    WHERE
        o.in_tx = a.main_ref;
$BODY$;

CREATE PROCEDURE midgard_agg.actions_add_fees(t1 bigint, t2 bigint)
LANGUAGE SQL AS $BODY$
    UPDATE midgard_agg.actions AS a
    SET
        fees = a.fees || f.fees
    FROM (
        SELECT
            tx,
            jsonb_agg(jsonb_build_object('asset', asset, 'amount', asset_e8)) AS fees
        FROM fee_events
        WHERE t1 <= block_timestamp AND block_timestamp < t2
        GROUP BY tx
        ) AS f
    WHERE
        f.tx = a.main_ref;
$BODY$;

CREATE PROCEDURE midgard_agg.update_actions_interval(t1 bigint, t2 bigint)
LANGUAGE SQL AS $BODY$
    CALL midgard_agg.insert_actions(t1, t2);
    CALL midgard_agg.trim_pending_actions(t1, t2);
    CALL midgard_agg.set_actions_height(t1, t2);
    CALL midgard_agg.actions_add_outbounds(t1, t2);
    CALL midgard_agg.actions_add_fees(t1, t2);
$BODY$;

INSERT INTO midgard_agg.watermarks (materialized_table, watermark)
    VALUES ('actions', 0);

CREATE PROCEDURE midgard_agg.update_actions(w_new bigint)
LANGUAGE plpgsql AS $BODY$
DECLARE
    w_old bigint;
BEGIN
    SELECT watermark FROM midgard_agg.watermarks WHERE materialized_table = 'actions'
        FOR UPDATE INTO w_old;
    IF w_new <= w_old THEN
        RAISE WARNING 'Updating actions into past: % -> %', w_old, w_new;
        RETURN;
    END IF;
    CALL midgard_agg.update_actions_interval(w_old, w_new);
    UPDATE midgard_agg.watermarks SET watermark = w_new WHERE materialized_table = 'actions';
END
$BODY$;

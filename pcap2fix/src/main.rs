// SPDX-License-Identifier: AGPL-3.0-only
// Minimal PCAP-to-FIX filter: reads PCAP (file or stdin), reassembles TCP
// streams, and emits FIX messages separated by the chosen delimiter.

use anyhow::{anyhow, Context, Result};
use clap::Parser;
use etherparse::{NetSlice, SlicedPacket, TransportSlice};
use pcap_parser::data::{get_packetdata, PacketData, ETHERTYPE_IPV4, ETHERTYPE_IPV6};
use pcap_parser::pcapng::Block;
use pcap_parser::traits::{PcapNGPacketBlock, PcapReaderIterator};
use pcap_parser::{create_reader, Linktype, PcapBlockOwned};
use std::collections::HashMap;
use std::env;
use std::fs::File;
use std::io::{self, Read, Write};
use std::net::{IpAddr, Ipv6Addr};
use std::time::{Duration, Instant};
use thiserror::Error;

const DEBUG_PCAP_PATH: &str = "/tmp/pcap_debug.in";
const DEBUG_FIX_PATH: &str = "/tmp/fix_debug.out";
const DEBUG_LOG_INTERVAL: Duration = Duration::from_secs(2);

#[derive(Parser, Debug)]
#[command(author, version, about)]
struct Args {
    /// PCAP file path or "-" for stdin
    #[arg(short, long, default_value = "-")]
    input: String,
    /// TCP port filter (optional). If omitted, all ports are considered.
    #[arg(short = 'p', long)]
    port: Option<u16>,
    /// Message delimiter. Accepts "SOH", literal char, or hex like \x01.
    #[arg(short = 'd', long, default_value = "SOH")]
    delimiter: String,
    /// Max bytes to buffer per flow before eviction
    #[arg(long, default_value = "1048576")]
    max_flow_bytes: usize,
    /// Idle timeout for flows (seconds)
    #[arg(long, default_value = "60")]
    idle_timeout: u64,
    /// Write raw PCAP and decoded FIX to debug files
    #[arg(long, default_value_t = false)]
    debug: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
struct FlowKey {
    src: IpAddr,
    dst: IpAddr,
    sport: u16,
    dport: u16,
    // direction handled by seq tracking in FlowState
}

#[derive(Debug)]
struct FlowState {
    next_seq: Option<u32>,
    buffer: Vec<u8>,
    last_seen: Instant,
}

impl Default for FlowState {
    fn default() -> Self {
        FlowState {
            next_seq: None,
            buffer: Vec::new(),
            last_seen: Instant::now(),
        }
    }
}

#[derive(Error, Debug)]
enum ReassemblyError {
    #[error("flow exceeded max buffer")]
    Overflow,
}

fn env_debug_enabled() -> bool {
    match env::var("PCAP2FIX_DEBUG") {
        Ok(val) => {
            let v = val.trim().to_ascii_lowercase();
            if v.is_empty() {
                true
            } else {
                !matches!(v.as_str(), "0" | "false" | "no" | "off")
            }
        }
        Err(_) => false,
    }
}

fn main() -> Result<()> {
    let args = Args::parse();
    let debug = args.debug || env_debug_enabled();
    let delimiter = parse_delimiter(&args.delimiter)?;
    let mut reader = open_reader(&args.input, debug)?;

    let mut flows: HashMap<FlowKey, FlowState> = HashMap::new();
    let idle = Duration::from_secs(args.idle_timeout);
    let stdout = io::BufWriter::new(io::stdout().lock());
    let mut debug_out: Option<io::BufWriter<File>> = None;
    if debug {
        let dbg_file = File::create(DEBUG_FIX_PATH)
            .with_context(|| format!("failed to create debug output at {DEBUG_FIX_PATH}"))?;
        debug_out = Some(io::BufWriter::new(dbg_file));
        eprintln!("debug mode enabled: PCAP -> {DEBUG_PCAP_PATH}, decoded FIX -> {DEBUG_FIX_PATH}");
    }
    let mut stdout_writer: Box<dyn Write> = if let Some(debug_file) = debug_out {
        Box::new(TeeWriter::new(stdout, debug_file))
    } else {
        Box::new(stdout)
    };
    let mut scratch = Vec::new();
    let mut legacy_linktype = None;
    let mut idb_linktypes: HashMap<u32, Linktype> = HashMap::new();
    let mut next_if_id: u32 = 0;
    let mut packet_count: u64 = 0;
    let mut byte_count: u64 = 0;
    let mut fix_count: u64 = 0;
    let mut last_log = Instant::now();

    loop {
        match reader.next() {
            Ok((offset, block)) => {
                {
                    match block {
                        PcapBlockOwned::LegacyHeader(hdr) => {
                            legacy_linktype = Some(hdr.network);
                        }
                        PcapBlockOwned::Legacy(b) => {
                            let linktype = legacy_linktype.unwrap_or(Linktype::ETHERNET);
                            if let Some(packet) =
                                get_packetdata(b.data, linktype, b.caplen as usize)
                            {
                                match handle_packet_data(
                                    packet,
                                    args.port,
                                    delimiter,
                                    args.max_flow_bytes,
                                    &mut flows,
                                    stdout_writer.as_mut(),
                                ) {
                                    Ok(messages) => {
                                        packet_count += 1;
                                        byte_count += b.caplen as u64;
                                        fix_count += messages as u64;
                                    }
                                    Err(err) => {
                                        eprintln!("warn: skipping packet: {err}");
                                    }
                                }
                            }
                        }
                        PcapBlockOwned::NG(block) => match block {
                            Block::SectionHeader(_) => {
                                idb_linktypes.clear();
                                next_if_id = 0;
                            }
                            Block::InterfaceDescription(idb) => {
                                idb_linktypes.insert(next_if_id, idb.linktype);
                                next_if_id += 1;
                            }
                            Block::EnhancedPacket(epb) => {
                                if let Some(linktype) = idb_linktypes.get(&epb.if_id) {
                                    if let Some(packet) = get_packetdata(
                                        epb.packet_data(),
                                        *linktype,
                                        epb.caplen as usize,
                                    ) {
                                        match handle_packet_data(
                                            packet,
                                            args.port,
                                            delimiter,
                                            args.max_flow_bytes,
                                            &mut flows,
                                            stdout_writer.as_mut(),
                                        ) {
                                            Ok(messages) => {
                                                packet_count += 1;
                                                byte_count += epb.caplen as u64;
                                                fix_count += messages as u64;
                                            }
                                            Err(err) => {
                                                eprintln!("warn: skipping packet: {err}");
                                            }
                                        }
                                    }
                                }
                            }
                            Block::SimplePacket(spb) => {
                                if let Some(linktype) = idb_linktypes.get(&0) {
                                    if let Some(packet) = get_packetdata(
                                        spb.packet_data(),
                                        *linktype,
                                        spb.origlen as usize,
                                    ) {
                                        match handle_packet_data(
                                            packet,
                                            args.port,
                                            delimiter,
                                            args.max_flow_bytes,
                                            &mut flows,
                                            stdout_writer.as_mut(),
                                        ) {
                                            Ok(messages) => {
                                                packet_count += 1;
                                                byte_count += spb.origlen as u64;
                                                fix_count += messages as u64;
                                            }
                                            Err(err) => {
                                                eprintln!("warn: skipping packet: {err}");
                                            }
                                        }
                                    }
                                }
                            }
                            _ => {}
                        },
                    }
                }
                reader.consume(offset);
                evict_idle(&mut flows, idle);
                if debug && last_log.elapsed() >= DEBUG_LOG_INTERVAL {
                    eprintln!(
                        "debug stats: packets={} bytes={} fix_messages={}",
                        packet_count, byte_count, fix_count
                    );
                    last_log = Instant::now();
                }
            }
            Err(pcap_parser::PcapError::Eof) => break,
            Err(pcap_parser::PcapError::Incomplete) => {
                // need more data
                reader
                    .refill()
                    .map_err(|e| anyhow!("failed to refill reader: {e}"))?;
            }
            Err(e) => return Err(anyhow!("pcap parse error: {e}")),
        }
    }

    // flush any trailing message fragments (best effort)
    for flow in flows.values_mut() {
        flush_complete_messages(
            &mut flow.buffer,
            delimiter,
            &mut scratch,
            stdout_writer.as_mut(),
        )?;
    }
    stdout_writer.flush()?;
    Ok(())
}

fn open_reader(path: &str, debug: bool) -> Result<Box<dyn PcapReaderIterator>> {
    if path == "-" {
        let stdin = io::stdin();
        if debug {
            let dbg = File::create(DEBUG_PCAP_PATH)
                .with_context(|| format!("failed to write debug PCAP to {DEBUG_PCAP_PATH}"))?;
            create_reader(65536, TeeReader::new(stdin.lock(), dbg))
                .map_err(|e| anyhow!("failed to create reader: {e}"))
        } else {
            create_reader(65536, stdin).map_err(|e| anyhow!("failed to create reader: {e}"))
        }
    } else {
        let file = File::open(path).with_context(|| format!("open pcap {path}"))?;
        if debug {
            std::fs::copy(path, DEBUG_PCAP_PATH)
                .with_context(|| format!("failed to write debug PCAP to {DEBUG_PCAP_PATH}"))?;
        }
        create_reader(65536, file).map_err(|e| anyhow!("failed to create reader: {e}"))
    }
}

struct TeeReader<R: Read, W: Write> {
    reader: R,
    debug: W,
}

impl<R: Read, W: Write> TeeReader<R, W> {
    fn new(reader: R, debug: W) -> Self {
        Self { reader, debug }
    }
}

impl<R: Read, W: Write> Read for TeeReader<R, W> {
    fn read(&mut self, buf: &mut [u8]) -> io::Result<usize> {
        let n = self.reader.read(buf)?;
        if n > 0 {
            self.debug.write_all(&buf[..n])?;
        }
        Ok(n)
    }
}

fn parse_delimiter(raw: &str) -> Result<u8> {
    if raw.eq_ignore_ascii_case("SOH") {
        return Ok(0x01);
    }
    if let Some(hex) = raw.strip_prefix("\\x").or_else(|| raw.strip_prefix("0x")) {
        let val =
            u8::from_str_radix(hex, 16).map_err(|_| anyhow!("invalid hex delimiter: {raw}"))?;
        return Ok(val);
    }
    if raw.len() == 1 {
        return Ok(raw.as_bytes()[0]);
    }
    Err(anyhow!(
        "delimiter must be SOH, hex (\\x01), or single byte"
    ))
}

fn handle_packet_data(
    packet: PacketData<'_>,
    port_filter: Option<u16>,
    delimiter: u8,
    max_flow_bytes: usize,
    flows: &mut HashMap<FlowKey, FlowState>,
    out: &mut dyn Write,
) -> Result<usize> {
    match packet {
        PacketData::L2(data) => {
            let sliced = SlicedPacket::from_ethernet(data).map_err(|e| anyhow!("parse: {e:?}"))?;
            handle_sliced_packet(sliced, port_filter, delimiter, max_flow_bytes, flows, out)
        }
        PacketData::L3(ethertype, data)
            if ethertype == ETHERTYPE_IPV4 || ethertype == ETHERTYPE_IPV6 =>
        {
            let sliced = SlicedPacket::from_ip(data).map_err(|e| anyhow!("parse: {e:?}"))?;
            handle_sliced_packet(sliced, port_filter, delimiter, max_flow_bytes, flows, out)
        }
        _ => Ok(0),
    }
}

fn handle_sliced_packet(
    sliced: SlicedPacket<'_>,
    port_filter: Option<u16>,
    delimiter: u8,
    max_flow_bytes: usize,
    flows: &mut HashMap<FlowKey, FlowState>,
    out: &mut dyn Write,
) -> Result<usize> {
    let (src_ip, dst_ip, tcp) = match (sliced.net, sliced.transport) {
        (Some(NetSlice::Ipv4(ip)), Some(TransportSlice::Tcp(tcp))) => (
            IpAddr::V4(ip.header().source_addr()),
            IpAddr::V4(ip.header().destination_addr()),
            tcp,
        ),
        (Some(NetSlice::Ipv6(ip)), Some(TransportSlice::Tcp(tcp))) => (
            IpAddr::V6(Ipv6Addr::from(ip.header().source())),
            IpAddr::V6(Ipv6Addr::from(ip.header().destination())),
            tcp,
        ),
        _ => return Ok(0),
    };
    if let Some(p) = port_filter {
        if tcp.source_port() != p && tcp.destination_port() != p {
            return Ok(0);
        }
    }

    let payload = tcp.payload();
    if payload.is_empty() {
        return Ok(0);
    }

    let key = FlowKey {
        src: src_ip,
        dst: dst_ip,
        sport: tcp.source_port(),
        dport: tcp.destination_port(),
    };

    let seq = tcp.sequence_number();
    let flow = flows.entry(key).or_default();
    flow.last_seen = Instant::now();

    reassemble_and_emit(flow, seq, payload, delimiter, max_flow_bytes, out)
}

fn reassemble_and_emit(
    flow: &mut FlowState,
    seq: u32,
    payload: &[u8],
    delimiter: u8,
    max_flow_bytes: usize,
    out: &mut dyn Write,
) -> Result<usize> {
    let expected = flow.next_seq.unwrap_or(seq);

    if seq == expected {
        flow.buffer.extend_from_slice(payload);
        flow.next_seq = Some(seq.wrapping_add(payload.len() as u32));
    } else if seq > expected {
        // out-of-order future segment: skip for now
        return Ok(0);
    } else {
        // retransmit or overlap
        let end = seq.wrapping_add(payload.len() as u32);
        if end <= expected {
            // fully duplicate
            return Ok(0);
        }
        let overlap = (expected - seq) as usize;
        flow.buffer.extend_from_slice(&payload[overlap..]);
        flow.next_seq = Some(expected.wrapping_add(payload.len() as u32 - overlap as u32));
    }

    if flow.buffer.len() > max_flow_bytes {
        flow.buffer.clear();
        return Err(ReassemblyError::Overflow.into());
    }

    let mut scratch = Vec::new();
    flush_complete_messages(&mut flow.buffer, delimiter, &mut scratch, out)
}

fn flush_complete_messages(
    buffer: &mut Vec<u8>,
    delimiter: u8,
    scratch: &mut Vec<u8>,
    out: &mut dyn Write,
) -> Result<usize> {
    let mut cursor = 0;
    let mut messages = 0;
    while let Some(rel_end) = find_message_end(&buffer[cursor..], delimiter) {
        let end = cursor + rel_end;
        scratch.clear();
        scratch.extend_from_slice(&buffer[cursor..=end]);
        scratch.push(b'\n'); // newline so each FIX message prints on its own line
        out.write_all(scratch)?;
        messages += 1;
        cursor = end + 1;
    }
    if cursor > 0 {
        buffer.drain(0..cursor);
    }
    Ok(messages)
}

fn find_message_end(buffer: &[u8], delimiter: u8) -> Option<usize> {
    // Need at least "8=..|9=..|" plus checksum ("10=000|")
    if buffer.len() < 16 {
        return None;
    }
    let begin_end = buffer.iter().position(|b| *b == delimiter)?;
    let body_len_field_start = begin_end + 1;
    let body_len_end = body_len_field_start
        + buffer[body_len_field_start..]
            .iter()
            .position(|b| *b == delimiter)?; // include delimiter
    if body_len_end <= body_len_field_start + 1 {
        return None;
    }
    if !buffer[body_len_field_start..].starts_with(b"9=") {
        return None;
    }
    let body_len_bytes = &buffer[body_len_field_start + 2..body_len_end];
    let body_len: usize = parse_decimal(body_len_bytes)?;
    let body_start = body_len_end + 1;
    let body_end = body_start.checked_add(body_len)?;
    // checksum starts immediately after body
    if body_end + 7 > buffer.len() {
        return None;
    }
    if !buffer.get(body_end..)?.starts_with(b"10=") {
        return None;
    }
    let checksum_val = buffer.get(body_end + 3..body_end + 6)?;
    if checksum_val.iter().any(|b| !b.is_ascii_digit()) {
        return None;
    }
    let end_delim_idx = body_end + 6;
    if *buffer.get(end_delim_idx)? != delimiter {
        return None;
    }
    Some(end_delim_idx)
}

fn parse_decimal(bytes: &[u8]) -> Option<usize> {
    let mut val: usize = 0;
    for b in bytes {
        if !b.is_ascii_digit() {
            return None;
        }
        val = val.checked_mul(10)?;
        val = val.checked_add((b - b'0') as usize)?;
    }
    Some(val)
}
fn evict_idle(flows: &mut HashMap<FlowKey, FlowState>, idle: Duration) {
    let now = Instant::now();
    flows.retain(|_, state| now.duration_since(state.last_seen) < idle);
}

struct TeeWriter<W1: Write, W2: Write> {
    primary: W1,
    debug: W2,
}

impl<W1: Write, W2: Write> TeeWriter<W1, W2> {
    fn new(primary: W1, debug: W2) -> Self {
        Self { primary, debug }
    }
}

impl<W1: Write, W2: Write> Write for TeeWriter<W1, W2> {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        // Always attempt to write the full buffer to both sinks; return the input length.
        self.primary.write_all(buf)?;
        self.debug.write_all(buf)?;
        Ok(buf.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        self.primary.flush()?;
        self.debug.flush()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn build_fix_message(body: &str, delim: u8) -> Vec<u8> {
        let mut msg = Vec::new();
        let d = delim as char;
        let body_len = body.len();
        msg.extend_from_slice(format!("8=FIX.4.4{d}9={body_len}{d}").as_bytes());
        msg.extend_from_slice(body.as_bytes());
        let checksum: u8 = msg.iter().fold(0u16, |acc, b| acc + *b as u16) as u8;
        msg.extend_from_slice(format!("10={:03}{}", checksum, d).as_bytes());
        msg
    }

    #[test]
    fn parse_delimiter_variants() {
        assert_eq!(parse_delimiter("SOH").unwrap(), 0x01);
        assert_eq!(parse_delimiter("\\x02").unwrap(), 0x02);
        assert_eq!(parse_delimiter("0x03").unwrap(), 0x03);
        assert_eq!(parse_delimiter("|").unwrap(), b'|');
    }

    #[test]
    fn reassembly_appends_in_order() {
        let mut flow = FlowState::default();
        let mut out = Vec::new();
        let message = build_fix_message("35=0\u{0001}", 0x01);
        let (part1, rest) = message.split_at(10);
        let (part2, part3) = rest.split_at(8);

        reassemble_and_emit(&mut flow, 10, part1, 0x01, 1024, &mut out).unwrap();
        reassemble_and_emit(
            &mut flow,
            10 + part1.len() as u32,
            part2,
            0x01,
            1024,
            &mut out,
        )
        .unwrap();
        assert!(out.is_empty(), "no complete message yet");
        reassemble_and_emit(
            &mut flow,
            10 + (part1.len() + part2.len()) as u32,
            part3,
            0x01,
            1024,
            &mut out,
        )
        .unwrap();
        let text = String::from_utf8(out).unwrap();
        assert!(text.contains("8=FIX.4.4"));
        assert!(text.ends_with('\n'));
    }

    #[test]
    fn flushes_full_messages_only() {
        let mut buf = build_fix_message("35=0\u{0001}", 0x01);
        buf.extend_from_slice(b"extra");
        let mut out = Vec::new();
        let mut scratch = Vec::new();
        let msgs = flush_complete_messages(&mut buf, 0x01, &mut scratch, &mut out).unwrap();
        let mut expected = build_fix_message("35=0\u{0001}", 0x01);
        expected.push(b'\n');
        assert_eq!(out, expected);
        assert_eq!(buf.as_slice(), b"extra");
        assert_eq!(msgs, 1);
    }

    #[test]
    fn retransmit_is_ignored() {
        let mut flow = FlowState::default();
        let mut out = Vec::new();
        reassemble_and_emit(&mut flow, 1, b"ABC", b'|', 1024, &mut out).unwrap();
        reassemble_and_emit(&mut flow, 1, b"ABC", b'|', 1024, &mut out).unwrap();
        assert!(flow.buffer.starts_with(b"ABC"));
    }

    #[test]
    fn out_of_order_future_segment_is_skipped() {
        let mut flow = FlowState::default();
        let mut out = Vec::new();
        reassemble_and_emit(&mut flow, 5, b"first", b'|', 1024, &mut out).unwrap();
        // future seq skipped
        reassemble_and_emit(&mut flow, 20, b"second", b'|', 1024, &mut out).unwrap();
        assert_eq!(flow.buffer, b"first");
    }

    #[test]
    fn flush_complete_messages_emits_and_retains_tail() {
        let mut buf = Vec::new();
        let msg1 = build_fix_message("35=0|", b'|');
        let msg2 = build_fix_message("35=1|", b'|');
        buf.extend_from_slice(&msg1);
        buf.extend_from_slice(&msg2);
        buf.extend_from_slice(b"partial");
        let mut scratch = Vec::new();
        let mut out = Vec::new();
        let msgs = flush_complete_messages(&mut buf, b'|', &mut scratch, &mut out).unwrap();
        let expected_out = {
            let mut v = msg1.clone();
            v.push(b'\n');
            v.extend_from_slice(&msg2);
            v.push(b'\n');
            v
        };
        assert_eq!(out, expected_out);
        assert_eq!(buf, b"partial");
        assert_eq!(msgs, 2);
    }

    #[test]
    fn env_debug_enabled_respects_values() {
        env::set_var("PCAP2FIX_DEBUG", "1");
        assert!(env_debug_enabled());
        env::set_var("PCAP2FIX_DEBUG", "true");
        assert!(env_debug_enabled());
        env::set_var("PCAP2FIX_DEBUG", "false");
        assert!(!env_debug_enabled());
        env::set_var("PCAP2FIX_DEBUG", "0");
        assert!(!env_debug_enabled());
        env::remove_var("PCAP2FIX_DEBUG");
        assert!(!env_debug_enabled());
    }

    #[test]
    fn tee_writer_writes_to_both() {
        let mut primary = Vec::new();
        let mut debug = Vec::new();
        {
            let mut tee = TeeWriter::new(&mut primary, &mut debug);
            tee.write_all(b"hello").unwrap();
            tee.flush().unwrap();
        }
        assert_eq!(primary, b"hello");
        assert_eq!(debug, b"hello");
    }
}
